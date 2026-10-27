package taskplan

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"chaos-go/internal/platform/deepseek"

	"github.com/robfig/cron/v3"
)

const (
	schedulerInterval = time.Minute
	cronLookahead     = 1
	cronSweepCap      = 50

	defaultRootPlanName = "每日任务"

	maxOriginalLen = 4096
	maxAnswerLen   = 1024
)

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

func filterPlansWithAncestors(plans []TaskPlan, keyword string) []TaskPlan {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return plans
	}
	lower := strings.ToLower(keyword)
	byID := make(map[int]TaskPlan, len(plans))
	for _, p := range plans {
		byID[p.ID] = p
	}

	keep := make(map[int]bool)
	markWithAncestors := func(start TaskPlan) {
		cur := start
		for {
			if keep[cur.ID] {
				break
			}
			keep[cur.ID] = true
			if cur.ParentID == nil {
				break
			}
			parent, ok := byID[*cur.ParentID]
			if !ok {
				break
			}
			cur = parent
		}
	}
	for _, p := range plans {
		name := strings.ToLower(p.Name)
		remark := ""
		if p.Remark != nil {
			remark = strings.ToLower(*p.Remark)
		}
		if strings.Contains(name, lower) || strings.Contains(remark, lower) {
			markWithAncestors(p)
		}
	}

	filtered := make([]TaskPlan, 0, len(keep))
	for _, p := range plans {
		if keep[p.ID] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

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

			result := NextReview(CardFromPlan(plan), now, *rating)
			task.StartedAt = &result.Due
			result.ApplyToPlan(plan, now)
			plan.UpdatedAt = now
		}
	}

	if err := CreateTask(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

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

func CreatePlan(req CreatePlanRequest) (*PlanResponse, error) {
	planType := req.PlanType
	if planType == "" {
		planType = TaskPlanTypeTodo
	}
	if planType == TaskPlanTypeTodo && req.StartedAt == nil {
		return nil, &InvalidStateError{Msg: "待办类型必须传 startedAt"}
	}
	orderNum := 0
	if req.OrderNum != nil {
		orderNum = *req.OrderNum
	}
	priority := 5
	if req.Priority != nil {
		priority = *req.Priority
	}

	plan := TaskPlan{
		ParentID: req.ParentID,
		Name:     req.Name,
		Status:   TaskPlanStatusCreated,
		PlanType: planType,
		CronExpr: req.CronExpr,
		OrderNum: orderNum,
		Priority: priority,
		Remark:   req.Remark,
		Link:     req.Link,
	}
	if err := CreateTaskPlanRow(&plan); err != nil {
		return nil, fmt.Errorf("创建任务计划失败: %w", err)
	}

	resp := &PlanResponse{
		ID:        plan.ID,
		ParentID:  plan.ParentID,
		Name:      plan.Name,
		Status:    plan.Status,
		PlanType:  plan.PlanType,
		CronExpr:  plan.CronExpr,
		OrderNum:  plan.OrderNum,
		Priority:  plan.Priority,
		Remark:    plan.Remark,
		Link:      plan.Link,
		CreatedAt: plan.CreatedAt,
		UpdatedAt: plan.UpdatedAt,
	}
	if plan.PlanType == TaskPlanTypeTodo {
		firstTask, err := generateTask(&plan, *req.StartedAt, nil)
		if err != nil {
			return nil, fmt.Errorf("生成任务失败: %w", err)
		}
		resp.FirstTask = taskToView(*firstTask)
	}
	return resp, nil
}

func UpdatePlan(id int, req UpdatePlanRequest) (*TaskPlan, error) {
	if _, err := FindActiveTaskPlan(id); err != nil {
		return nil, err
	}
	updated := map[string]interface{}{}
	if req.Name != nil {
		updated["name"] = *req.Name
	}
	if req.ParentID != nil {
		updated["parent_id"] = *req.ParentID
	} else {
		updated["parent_id"] = nil
	}
	updated["plan_type"] = req.PlanType
	if req.OrderNum != nil {
		updated["order_num"] = *req.OrderNum
	}
	if req.Priority != nil {
		updated["priority"] = *req.Priority
	}
	if req.Remark != nil {
		updated["remark"] = *req.Remark
	}
	if req.Link != nil {
		updated["link"] = *req.Link
	}
	if req.CronExpr != nil {
		updated["cron_expr"] = *req.CronExpr
	}
	if len(updated) == 0 {
		return GetTaskPlanByID(id)
	}
	updated["updated_at"] = time.Now()
	if err := UpdateTaskPlanColumns(id, updated); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	plan, err := GetTaskPlanByID(id)
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}
	return plan, nil
}

func StartPlan(id int) (*PlanStateResponse, error) {
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.Status != TaskPlanStatusCreated && plan.Status != TaskPlanStatusStarted {
		return nil, &InvalidStateError{Msg: "当前状态不允许开启"}
	}
	if plan.IsSuspended {
		return nil, &InvalidStateError{Msg: "计划已挂起，请先恢复再开启"}
	}

	plan.Status = TaskPlanStatusStarted
	plan.UpdatedAt = time.Now()
	if err := SaveTaskPlan(plan); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}

	activeCount, err := CountActiveTasks(plan.ID)
	if err != nil {
		return nil, fmt.Errorf("检查任务失败: %w", err)
	}
	resp := &PlanStateResponse{
		ID:        plan.ID,
		Name:      plan.Name,
		Status:    plan.Status,
		PlanType:  plan.PlanType,
		UpdatedAt: plan.UpdatedAt,
	}
	if activeCount == 0 {
		respTask, err := generateTask(plan, time.Now(), nil)
		if err != nil {
			return nil, fmt.Errorf("生成任务失败: %w", err)
		}
		resp.FirstTask = taskToView(*respTask)
	}
	return resp, nil
}

func CompletePlan(id int) (*TaskPlan, error) {
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.Status != TaskPlanStatusStarted {
		return nil, &InvalidStateError{Msg: "当前状态不允许完成"}
	}
	now := time.Now()
	plan.Status = TaskPlanStatusCompleted
	plan.UpdatedAt = now
	if err := CompleteActiveTasksForPlan(id, now); err != nil {
		return nil, fmt.Errorf("更新任务失败: %w", err)
	}
	return plan, nil
}

func ArchivePlan(id int) (*TaskPlan, error) {
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.Status != TaskPlanStatusCompleted {
		return nil, &InvalidStateError{Msg: "当前状态不允许归档"}
	}
	plan.Status = TaskPlanStatusArchived
	plan.UpdatedAt = time.Now()
	if err := SaveTaskPlan(plan); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return plan, nil
}

func DeletePlan(id int, cascade bool) (*DeletePlanResponse, error) {
	if _, err := FindActiveTaskPlan(id); err != nil {
		return nil, err
	}
	if !cascade {
		childCount, err := CountChildPlans(id)
		if err != nil {
			return nil, fmt.Errorf("检查子任务失败: %w", err)
		}
		activeTaskCount, err := CountActiveTasks(id)
		if err != nil {
			return nil, fmt.Errorf("检查任务失败: %w", err)
		}
		if childCount > 0 {
			return nil, &InvalidStateError{Msg: "存在子任务计划，无法删除"}
		}
		if activeTaskCount > 0 {
			return nil, &InvalidStateError{Msg: "存在未完成的任务，无法删除"}
		}
	}
	deletedPlanCount, err := SoftDeletePlanAndTasks(id, cascade)
	if err != nil {
		return nil, fmt.Errorf("删除任务计划失败: %w", err)
	}
	resp := &DeletePlanResponse{Message: "删除成功", Cascade: cascade}
	if cascade {
		resp.DeletedPlanCount = &deletedPlanCount
	}
	return resp, nil
}

func SuspendPlans(id int) error {
	return updateDescendants(id, map[string]interface{}{"is_suspended": true}, "挂起失败")
}

func ResumePlans(id int) error {
	return updateDescendants(id, map[string]interface{}{"is_suspended": false}, "恢复失败")
}

func SetPlanPriority(id int, priority int) error {
	return updateDescendants(id, map[string]interface{}{"priority": priority}, "更新优先级失败")
}

func updateDescendants(id int, columns map[string]interface{}, errMsg string) error {
	if _, err := FindActiveTaskPlan(id); err != nil {
		return err
	}
	ids, err := CollectDescendantPlanIDs(id)
	if err != nil {
		return fmt.Errorf("收集子任务失败: %w", err)
	}
	columns = cloneColumns(columns)
	columns["updated_at"] = time.Now()
	if err := UpdatePlanColumnsByIDs(ids, columns); err != nil {
		return fmt.Errorf("%s: %w", errMsg, err)
	}
	return nil
}

func cloneColumns(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src)+1)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func PlanTree(keyword string) ([]TaskPlanTree, error) {
	plans, err := ListPlanTreeRows()
	if err != nil {
		return nil, err
	}
	return buildTaskPlanTree(filterPlansWithAncestors(plans, keyword)), nil
}

func PlanTasks(id int) ([]Task, error) {
	if _, err := FindActiveTaskPlan(id); err != nil {
		return nil, err
	}
	return ListTasksByPlan(id)
}

func FetchPlanRaw(id int) (*PlanRaw, error) {
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.RawLink == nil || *plan.RawLink == "" {
		return nil, ErrNoRawLink
	}
	content, err := fetchRawContent(*plan.RawLink)
	if err != nil {
		return nil, fmt.Errorf("获取原文失败: %w", err)
	}
	return &PlanRaw{RawLink: *plan.RawLink, Content: content}, nil
}

func CompleteTaskByID(id int, rating *int) (TaskView, error) {
	task, err := FindActiveTask(id)
	if err != nil {
		return nil, err
	}
	if task.Status != TaskStatusActive {
		return nil, &InvalidStateError{Msg: "任务已完成"}
	}
	plan, err := FindActiveTaskPlan(task.PlanID)
	if err != nil {
		return nil, err
	}

	var fsrsRating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		if rating == nil {
			return nil, &InvalidStateError{Msg: "interval 类型任务必须传 rating，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"}
		}
		r := FsrsRating(*rating)
		if r < RatingAgain || r > RatingEasy {
			return nil, &InvalidStateError{Msg: "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"}
		}
		fsrsRating = &r
	}

	nextTask, err := finishTask(task, plan, fsrsRating)
	if err != nil {
		return nil, fmt.Errorf("完成任务失败: %w", err)
	}
	resp := taskToView(*task)
	if nextTask != nil {
		resp["nextTask"] = taskToView(*nextTask)
	}
	return resp, nil
}

func PostponeTaskByID(id, days int) error {
	task, err := FindActiveTask(id)
	if err != nil {
		return err
	}
	updates, ok, reason := computePostponeUpdates(task, days)
	if !ok {
		return &InvalidStateError{Msg: reason}
	}
	if err := UpdateTaskColumns(task.ID, updates); err != nil {
		return fmt.Errorf("延期失败: %w", err)
	}
	return nil
}

func PostponeTasks(ids []int, days int) (*BatchPostponeResult, error) {
	res := &BatchPostponeResult{Results: make([]PostponeItemResult, 0, len(ids))}
	for _, id := range ids {
		task, err := FindActiveTask(id)
		if err != nil {
			res.Results = append(res.Results, PostponeItemResult{ID: id, Status: "skipped", Reason: "任务不存在"})
			res.Skipped++
			continue
		}
		updates, ok, reason := computePostponeUpdates(task, days)
		if !ok {
			res.Results = append(res.Results, PostponeItemResult{ID: id, Status: "skipped", Reason: reason})
			res.Skipped++
			continue
		}
		if err := UpdateTaskColumns(id, updates); err != nil {
			return nil, fmt.Errorf("延期失败: %w", err)
		}
		res.Results = append(res.Results, PostponeItemResult{ID: id, Status: "postponed"})
		res.Postponed++
	}
	return res, nil
}

func CancelTaskByID(id int) (TaskView, error) {
	task, err := FindActiveTask(id)
	if err != nil {
		return nil, err
	}
	if task.Status != TaskStatusActive {
		return nil, &InvalidStateError{Msg: "任务已完成或已取消"}
	}
	plan, err := FindActiveTaskPlan(task.PlanID)
	if err != nil {
		return nil, err
	}
	if plan.PlanType != TaskPlanTypeCron && plan.PlanType != TaskPlanTypeInterval {
		return nil, &InvalidStateError{Msg: "仅周期重复任务支持取消"}
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.CompletedAt = &now
	if err := SaveTask(task); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	resp := taskToView(*task)
	nextTask, err := generateTask(plan, now, nil)
	if err != nil {
		return nil, fmt.Errorf("生成下一条任务失败: %w", err)
	}
	resp["nextTask"] = taskToView(*nextTask)
	return resp, nil
}

const reviewScoreSystemPrompt = `你是一位严谨的「主动回忆(active recall)」学习评分员。用户会提供两段内容：
1) 【原文】：需要学习/记忆的材料。
2) 【我的回忆】：用户在未看原文情况下回忆写下的内容（关键词、要点或短句）。

你的任务：
- 从【原文】中提炼 3~5 个「必须记住的关键点」，覆盖核心概念、方法、结论或易错点；不要超过 5 个，尽量精炼。
- 逐条判断【我的回忆】是否覆盖了该关键点（同义、要点到位即算命中，不要求字字对应）。
- 计算覆盖度 coverage = 命中条数 / 总条数 * 100（整数）。
- 给出建议评分 suggestedRating（1~4 整数）：
  1=Again（几乎没想起来，覆盖度<40% 或关键结论错误）
  2=Hard（想起一部分、有明显遗漏，覆盖度 40%~69%）
  3=Good（基本完整、少量遗漏，覆盖度 70%~89%）
  4=Easy（完整且准确，覆盖度>=90%）。

只输出如下 JSON（不要任何额外文字、不要 markdown 代码块）：
{
  "points": [{"text":"关键点表述","covered":true,"reason":"一句简短说明命中/遗漏原因"}],
  "coverage": 80,
  "suggestedRating": 3
}`

func ReviewPlan(id int, req ReviewRequest) (*ReviewResponse, error) {
	if req.Rating < int(RatingAgain) || req.Rating > int(RatingEasy) {
		return nil, &InvalidStateError{Msg: "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy"}
	}
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.Status == TaskPlanStatusCompleted || plan.Status == TaskPlanStatusArchived {
		return nil, &InvalidStateError{Msg: "已完成/已归档的计划不可复习"}
	}

	var task Task
	if foundTask, ok := FindActiveTaskForPlan(id); ok {
		task = *foundTask
	} else {

		if plan.Status != TaskPlanStatusStarted && plan.Status != TaskPlanStatusCreated {
			return nil, &InvalidStateError{Msg: "当前状态无法发起复习"}
		}
		generated, err := generateTask(plan, time.Now(), nil)
		if err != nil {
			return nil, fmt.Errorf("生成复习任务失败: %w", err)
		}
		task = *generated
	}

	remarkObj := map[string]interface{}{"answer": req.Answer}
	if req.AI != nil {
		remarkObj["ai"] = req.AI
	}
	if remarkBytes, merr := json.Marshal(remarkObj); merr == nil {
		remarkStr := string(remarkBytes)
		task.Remark = &remarkStr
	} else {
		ans := req.Answer
		task.Remark = &ans
	}

	var rating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		r := FsrsRating(req.Rating)
		rating = &r
	}

	nextTask, err := finishTask(&task, plan, rating)
	if err != nil {
		return nil, fmt.Errorf("复习失败: %w", err)
	}
	resp := &ReviewResponse{
		PlanID:         plan.ID,
		Name:           plan.Name,
		FsrsState:      plan.FsrsState,
		FsrsStability:  plan.FsrsStability,
		FsrsDifficulty: plan.FsrsDifficulty,
		FsrsReps:       plan.FsrsReps,
		FsrsLapses:     plan.FsrsLapses,
	}
	if nextTask != nil && nextTask.StartedAt != nil {
		resp.NextReviewAt = nextTask.StartedAt
	}
	return resp, nil
}

func ScoreReview(original, answer string) (*ReviewScore, error) {
	original = strings.TrimSpace(original)
	answer = strings.TrimSpace(answer)
	if original == "" {
		return nil, &InvalidStateError{Msg: "缺少原文内容，无法评分"}
	}

	if len(original) > maxOriginalLen {
		original = original[:maxOriginalLen] + "\n…（原文过长已截断）"
	}
	if len(answer) > maxAnswerLen {
		answer = answer[:maxAnswerLen] + "\n…（答案过长已截断）"
	}

	userMsg := "【原文】\n" + original + "\n\n【我的回忆】\n" + answer
	raw, err := deepseek.Chat(reviewScoreSystemPrompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("AI 评分失败: %w", err)
	}

	var score ReviewScore
	if err := json.Unmarshal([]byte(raw), &score); err != nil {
		return nil, fmt.Errorf("AI 返回解析失败")
	}

	if score.Coverage < 0 {
		score.Coverage = 0
	}
	if score.Coverage > 100 {
		score.Coverage = 100
	}
	if score.SuggestedRating < 1 || score.SuggestedRating > 4 {
		switch {
		case score.Coverage >= 90:
			score.SuggestedRating = 4
		case score.Coverage >= 70:
			score.SuggestedRating = 3
		case score.Coverage >= 40:
			score.SuggestedRating = 2
		default:
			score.SuggestedRating = 1
		}
	}
	return &score, nil
}

func DailyStats(days int) ([]DailyPoint, error) {
	today := todayStart()
	cutoff := today.AddDate(0, 0, -days)
	rows, err := DailyCompletionRows(cutoff)
	if err != nil {
		return nil, err
	}
	rowMap := countByLocalDate(rows)
	result := make([]DailyPoint, 0, days+1)
	for i := days; i >= 0; i-- {
		key := today.AddDate(0, 0, -i).Format("2006-01-02")
		result = append(result, DailyPoint{Date: key, Count: rowMap[key]})
	}
	return result, nil
}

func ActiveStats(start, end time.Time) ([]DailyPoint, error) {
	rows, err := ActiveStartRows(start, end)
	if err != nil {
		return nil, err
	}
	rowMap := countByLocalDate(rows)
	result := make([]DailyPoint, 0, 30)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		result = append(result, DailyPoint{Date: key, Count: rowMap[key]})
	}
	return result, nil
}

func BuildContributionStats(planID int, rootName string) (*ContributionStats, error) {
	today := todayStart()

	start := today.AddDate(-1, 0, 1)

	var roots []TaskPlan
	if planID > 0 {
		plan, err := FindActiveUnsuspendedPlanByID(planID)
		if err != nil {
			return nil, err
		}
		roots = append(roots, *plan)
	} else {
		if strings.TrimSpace(rootName) == "" {
			rootName = defaultRootPlanName
		}
		plans, err := FindActiveUnsuspendedPlansByName(rootName)
		if err != nil {
			return nil, fmt.Errorf("查询计划失败: %w", err)
		}
		roots = plans
	}

	options := make([]TaskPlan, 0, 8)
	resolvedRootName := ""
	for _, root := range roots {
		if resolvedRootName == "" {
			resolvedRootName = root.Name
		}
		children, err := ListChildren(root.ID)
		if err != nil {
			return nil, fmt.Errorf("查询子计划失败: %w", err)
		}
		if len(children) == 0 {
			options = append(options, root)
		} else {
			options = append(options, children...)
		}
	}

	owner := make(map[int]int)
	for idx := range options {
		ids, err := CollectDescendantPlanIDs(options[idx].ID)
		if err != nil {
			return nil, fmt.Errorf("收集子计划失败: %w", err)
		}
		for _, id := range ids {
			owner[id] = idx
		}
	}

	counts := make([]map[string]int, len(options))
	for i := range counts {
		counts[i] = make(map[string]int)
	}
	if len(owner) > 0 {
		planIDs := make([]int, 0, len(owner))
		for id := range owner {
			planIDs = append(planIDs, id)
		}
		rows, err := ContributionRows(planIDs, start)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			idx, ok := owner[r.PlanID]
			if !ok {
				continue
			}
			key := localDateKey(r.StartedAt)
			counts[idx][key]++
		}
	}

	items := make([]ContributionItem, 0, len(options))
	for i, opt := range options {
		items = append(items, buildContributionItem(opt.ID, opt.Name, start, today, counts[i]))
	}
	return &ContributionStats{
		RootName: resolvedRootName,
		Start:    start.Format("2006-01-02"),
		End:      today.Format("2006-01-02"),
		Items:    items,
	}, nil
}

func buildContributionItem(id int, name string, start, today time.Time, counts map[string]int) ContributionItem {
	days := make([]DailyPoint, 0, 366)
	total := 0
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		count := counts[key]
		total += count
		days = append(days, DailyPoint{Date: key, Count: count})
	}
	return ContributionItem{ID: id, Name: name, Total: total, Days: days}
}

func todayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

func localDateKey(t time.Time) string {
	local := t.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02")
}

func countByLocalDate(rows []time.Time) map[string]int {
	rowMap := make(map[string]int, len(rows))
	for _, r := range rows {
		rowMap[localDateKey(r)]++
	}
	return rowMap
}
