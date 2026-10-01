package taskplan

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/robfig/cron/v3"
)

// fsrsInstance 全局 FSRS 调度器（默认参数）。
var fsrsInstance = NewFsrs(nil)

const (
	schedulerInterval = time.Minute
	cronLookahead     = 1
	cronSweepCap      = 50
)

// buildTaskPlanTree 由扁平计划列表组装成树（按 OrderNum/ID 稳定排序）。
func buildTaskPlanTree(plans []TaskPlan) []TaskPlanTree {
	childrenMap := make(map[int][]TaskPlan)
	var roots []TaskPlan

	for _, p := range plans {
		if p.ParentID == nil {
			roots = append(roots, p)
		} else {
			childrenMap[*p.ParentID] = append(childrenMap[*p.ParentID], p)
		}
	}

	sortByOrder := func(list []TaskPlan) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].OrderNum != list[j].OrderNum {
				return list[i].OrderNum < list[j].OrderNum
			}
			return list[i].ID < list[j].ID
		})
	}

	sortByOrder(roots)
	for _, children := range childrenMap {
		sortByOrder(children)
	}

	var build func(parent TaskPlan) TaskPlanTree
	build = func(parent TaskPlan) TaskPlanTree {
		node := TaskPlanTree{TaskPlan: parent}
		node.HasLink = parent.Link != nil
		node.Link = nil
		for _, child := range childrenMap[parent.ID] {
			node.Children = append(node.Children, build(child))
		}
		return node
	}

	result := make([]TaskPlanTree, 0, len(roots))
	for _, root := range roots {
		result = append(result, build(root))
	}
	return result
}

// generateTask 根据计划类型生成一条任务。
// cron：解析表达式并委托 createCronTask；interval：按 FSRS 推算下次到期并回写计划记忆参数；
// todo：直接以 now 作为 startedAt 创建。rating 仅 interval 类型生效。
func generateTask(plan *TaskPlan, now time.Time, rating *FsrsRating) (*Task, error) {
	now = now.Truncate(time.Second)
	task := Task{
		PlanID:    plan.ID,
		Status:    TaskStatusActive,
		StartedAt: &now,
	}

	switch plan.PlanType {
	case TaskPlanTypeCron:
		if plan.CronExpr == nil || *plan.CronExpr == "" {
			return nil, fmt.Errorf("cron表达式不能为空")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(*plan.CronExpr)
		if err != nil {
			return nil, fmt.Errorf("无效的cron表达式: %v", err)
		}
		return createCronTask(plan, schedule.Next(now))

	case TaskPlanTypeInterval:
		if rating != nil {
			fsrsCard := FsrsCard{
				Stability:     plan.FsrsStability,
				Difficulty:    plan.FsrsDifficulty,
				Reps:          plan.FsrsReps,
				Lapses:        plan.FsrsLapses,
				State:         FsrsState(plan.FsrsState),
				LearningSteps: plan.FsrsLearningSteps,
			}
			if plan.FsrsLastReviewAt != nil {
				fsrsCard.LastReview = plan.FsrsLastReviewAt
			}
			result := fsrsInstance.Next(&fsrsCard, now, *rating)
			task.StartedAt = &result.Due

			plan.FsrsStability = result.Card.Stability
			plan.FsrsDifficulty = result.Card.Difficulty
			plan.FsrsReps = result.Card.Reps
			plan.FsrsLapses = result.Card.Lapses
			plan.FsrsState = int(result.Card.State)
			plan.FsrsLearningSteps = result.Card.LearningSteps
			plan.FsrsLastReviewAt = &now
			plan.UpdatedAt = now
		}
	}

	if err := CreateTask(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

// finishTask 标记任务完成、驱动 FSRS（interval 类型）并生成下一条任务。
// 返回生成的下一条任务（可能为 nil）。task、plan 需已加载。
func finishTask(task *Task, plan *TaskPlan, rating *FsrsRating) (*Task, error) {
	now := time.Now()
	task.Status = TaskStatusDone
	task.CompletedAt = &now
	if err := SaveTask(task); err != nil {
		return nil, err
	}

	var nextTask *Task
	switch plan.PlanType {
	case TaskPlanTypeCron, TaskPlanTypeInterval:
		if plan.IsSuspended {
			break
		}
		generated, err := generateTask(plan, now, rating)
		if err != nil {
			return nil, err
		}
		nextTask = generated
	}

	if plan.PlanType == TaskPlanTypeInterval && rating != nil {
		if err := SaveTaskPlan(plan); err != nil {
			return nil, err
		}
	}

	return nextTask, nil
}

// computePostponeUpdates 计算单条任务的延期更新字段，返回 updates、是否可延期与跳过原因。
// 仅 active 状态且所属 plan 为 todo/interval 类型的任务可延期，平移 started_at 与 deadline。
func computePostponeUpdates(task *Task, days int) (map[string]interface{}, bool, string) {
	if task.Status != TaskStatusActive {
		return nil, false, "任务已完成或已取消"
	}
	plan, err := FindActiveTaskPlan(task.PlanID)
	if err != nil {
		return nil, false, "所属任务计划不存在"
	}
	if plan.PlanType != TaskPlanTypeTodo && plan.PlanType != TaskPlanTypeInterval {
		return nil, false, "仅待办和间隔类型任务支持延期"
	}
	offset := time.Duration(days) * 24 * time.Hour
	updates := map[string]interface{}{}
	if task.StartedAt != nil {
		updates["started_at"] = task.StartedAt.Add(offset)
	}
	if task.Deadline != nil {
		updates["deadline"] = task.Deadline.Add(offset)
	}
	if len(updates) == 0 {
		return nil, false, "任务没有可延期的时间"
	}
	return updates, true, ""
}

// ── 调度扫描 ──────────────────────────────────────────────────────

// SweepScheduledTaskPlans 扫描已开启的 cron / interval 计划并生成任务，
// 现由定时任务模块（/api/systemJobs/sweep）按 cron 触发，不再自动注册到后台调度器。
func SweepScheduledTaskPlans() {
	sweepCronPlans()
	sweepIntervalPlans()
}

func sweepCronPlans() {
	plans, err := ListScheduledPlans(TaskPlanTypeCron, TaskPlanStatusStarted)
	if err != nil {
		slog.Error("周期任务扫描失败", "err", err)
		return
	}
	now := time.Now()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for i := range plans {
		plan := plans[i]
		if plan.CronExpr == nil || *plan.CronExpr == "" {
			continue
		}
		schedule, err := parser.Parse(*plan.CronExpr)
		if err != nil {
			slog.Warn("计划的 cron 表达式无效", "planId", plan.ID, "err", err)
			continue
		}
		upcoming, _ := CountActiveUpcomingTasks(plan.ID, now)
		if upcoming >= cronLookahead {
			continue
		}
		base := now
		if last, err := FindLatestTaskByPlan(plan.ID); err == nil && last.StartedAt != nil {
			base = *last.StartedAt
		}
		generated := 0
		t := base
		for int(upcoming)+generated < cronLookahead && generated < cronSweepCap {
			next := schedule.Next(t)
			if !next.After(now) {
				t = next
				continue
			}
			if _, err := createCronTask(&plan, next); err != nil {
				slog.Warn("生成 cron 任务失败", "planId", plan.ID, "err", err)
				break
			}
			generated++
			t = next
		}
	}
}

func sweepIntervalPlans() {
	plans, err := ListScheduledPlans(TaskPlanTypeInterval, TaskPlanStatusStarted)
	if err != nil {
		slog.Error("间隔任务扫描失败", "err", err)
		return
	}
	now := time.Now()
	for i := range plans {
		plan := plans[i]
		active, _ := CountActiveTasks(plan.ID)
		if active > 0 {
			continue
		}
		if _, err := generateTask(&plan, now, nil); err != nil {
			slog.Warn("补充间隔任务失败", "planId", plan.ID, "err", err)
		}
	}
}

// createCronTask 为 cron 计划在指定 startedAt 创建一条任务（已存在则直接返回）。
func createCronTask(plan *TaskPlan, startedAt time.Time) (*Task, error) {
	if existing, ok := FindTaskAt(plan.ID, startedAt); ok {
		return existing, nil
	}
	endOfDay := time.Date(startedAt.Year(), startedAt.Month(), startedAt.Day(),
		23, 59, 59, 0, startedAt.Location())
	task := Task{
		PlanID:    plan.ID,
		Status:    TaskStatusActive,
		StartedAt: &startedAt,
		Deadline:  &endOfDay,
	}
	if err := CreateTask(&task); err != nil {
		return nil, err
	}
	return &task, nil
}
