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

	// defaultRootPlanName 贡献热力图未指定根计划时的默认名称。
	defaultRootPlanName = "每日任务"
	// AI 评分的输入长度上限（控制 token 规模）。
	maxOriginalLen = 4096
	maxAnswerLen   = 1024
)

// ── 树 / 任务生成 / 完成（领域层）────────────────────────────────

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

// filterPlansWithAncestors 按关键字过滤计划，并保留命中项的完整祖先链（保证树形结构不散）。
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
			// 评分驱动 FSRS：还原卡片 → 推进一次 → 下次到期作为新任务的 startedAt
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

// ── 调度扫描 ────────────────────────────────────────────────────

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

// ── 计划用例 ────────────────────────────────────────────────────

// CreatePlan 创建任务计划；待办类型须带 startedAt，并顺带生成首条任务。
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

// UpdatePlan 按字段映射更新计划，返回更新后的计划。
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

// StartPlan 开启计划：置为 started，若尚无进行中的任务则生成首条。
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

// CompletePlan 完成计划：仅 started 状态可完成，同时结束其进行中的任务。
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

// ArchivePlan 归档计划：仅 completed 状态可归档。
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

// DeletePlan 删除计划；非级联时若存在子计划或未完成任务则拒绝。
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

// SuspendPlans 挂起计划及其子孙。
func SuspendPlans(id int) error {
	return updateDescendants(id, map[string]interface{}{"is_suspended": true}, "挂起失败")
}

// ResumePlans 恢复计划及其子孙。
func ResumePlans(id int) error {
	return updateDescendants(id, map[string]interface{}{"is_suspended": false}, "恢复失败")
}

// SetPlanPriority 批量设置计划及其子孙的优先级。
func SetPlanPriority(id int, priority int) error {
	return updateDescendants(id, map[string]interface{}{"priority": priority}, "更新优先级失败")
}

// updateDescendants 收集子孙后批量更新给定列。
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

// PlanTree 返回计划树；keyword 非空时按名称 / 备注过滤并保留祖先链。
func PlanTree(keyword string) ([]TaskPlanTree, error) {
	plans, err := ListPlanTreeRows()
	if err != nil {
		return nil, err
	}
	return buildTaskPlanTree(filterPlansWithAncestors(plans, keyword)), nil
}

// PlanTasks 列出某计划下的任务。
func PlanTasks(id int) ([]Task, error) {
	if _, err := FindActiveTaskPlan(id); err != nil {
		return nil, err
	}
	return ListTasksByPlan(id)
}

// FetchPlanRaw 拉取计划关联原文（服务端代理，规避浏览器跨域）。
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

// ── 任务用例 ────────────────────────────────────────────────────

// CompleteTaskByID 完成任务：interval 类型须带评分（驱动 FSRS），返回任务视图（含下一条）。
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

// PostponeTaskByID 单条延期。
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

// PostponeTasks 批量延期：逐条处理，返回汇总与逐项结果（部分失败不影响其余项）。
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

// CancelTaskByID 取消任务（仅 cron / interval 支持），并生成下一条任务。
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

// ── 复习用例 ────────────────────────────────────────────────────

// reviewScoreSystemPrompt 是固定的角色与评分标准提示（每次调用相同，省 token）。
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

// ReviewPlan 提交复习：评分驱动 FSRS（interval 类型），回忆内容与 AI 结果写入任务 remark。
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
		// 没有进行中的任务：按需生成一条（等价于「现在就复习一次」）
		if plan.Status != TaskPlanStatusStarted && plan.Status != TaskPlanStatusCreated {
			return nil, &InvalidStateError{Msg: "当前状态无法发起复习"}
		}
		generated, err := generateTask(plan, time.Now(), nil)
		if err != nil {
			return nil, fmt.Errorf("生成复习任务失败: %w", err)
		}
		task = *generated
	}

	// 持久化用户的回忆内容 + AI 评分结果（JSON 格式写入 tasks.remark）
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

// ScoreReview 用 DeepSeek 对「回忆答案」相对「原文」做覆盖度评分。
func ScoreReview(original, answer string) (*ReviewScore, error) {
	original = strings.TrimSpace(original)
	answer = strings.TrimSpace(answer)
	if original == "" {
		return nil, &InvalidStateError{Msg: "缺少原文内容，无法评分"}
	}
	// 控制 token 规模，避免长文拖累延迟与费用
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
	// 规整建议档位到 FSRS 四档；异常时用覆盖度兜底映射
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

// ── 统计用例 ────────────────────────────────────────────────────

// DailyStats 返回最近 days 天（含今天）的每日完成数。
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

// ActiveStats 返回 [start, end] 区间内每日「任务开始」数。
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

// ContributionStats 返回近一年每日完成任务数（GitHub 风格贡献热力图）。
// 每天归属按任务的开始时间（started_at）划分，而非完成时间：定时 / 周期任务在当日达成
// 就应计入当日，即使实际完成动作跨到了第二天。
// planID > 0 时以该计划为根；否则按 rootName（默认「每日任务」）查找根计划。
// 统计项 = 根计划的直接子计划（每个子项单独统计，不合并）；子计划为空时退化为根计划自身。
func BuildContributionStats(planID int, rootName string) (*ContributionStats, error) {
	today := todayStart()
	// 往前多取一天，由前端对齐到整周（周日）后渲染
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

	// 归集各统计项的子树计划：planID -> 选项下标
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

// buildContributionItem 按天补齐 [start, today]，生成单个统计项的贡献序列。
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

// ── 统计辅助 ────────────────────────────────────────────────────

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
