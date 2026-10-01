package taskplan

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("taskPlan", Register)
}

// Register 把任务计划与待办任务全部路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 注意：本模块未走通用 crud，由专用 handler 直接接线，以承载 FSRS / 调度 / 树形等定制行为。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/taskPlans")
	g.POST("/", CreateTaskPlan)
	g.GET("/", ListTaskPlans)
	g.GET("/tree", GetTaskPlanTree)
	g.GET("/:id", GetTaskPlan)
	g.PATCH("/:id", UpdateTaskPlan)
	g.PATCH("/:id/start", StartTaskPlan)
	g.PATCH("/:id/complete", CompleteTaskPlan)
	g.PATCH("/:id/archive", ArchiveTaskPlan)
	g.PATCH("/:id/suspend", SuspendTaskPlan)
	g.PATCH("/:id/resume", ResumeTaskPlan)
	g.PATCH("/:id/priority", SetPriorityTaskPlan)
	g.DELETE("/:id", DeleteTaskPlan)
	g.GET("/:id/tasks", ListPlanTasks)
	g.GET("/:id/raw", GetTaskPlanRaw)
	g.POST("/:id/review", ReviewTaskPlan)

	ai := rg.Group("/ai")
	ai.POST("/review-score", AiReviewScore)

	tasks := rg.Group("/tasks")
	tasks.GET("/pending", GetPendingTasks)
	tasks.GET("/dailyStats", GetTaskDailyStats)
	tasks.GET("/activeStats", GetTaskActiveStats)
	tasks.GET("/contributionStats", GetTaskContributionStats)
	tasks.PATCH("/:id/complete", CompleteTask)
	tasks.PATCH("/:id/cancel", CancelTask)
	tasks.PATCH("/:id/postpone", PostponeTask)
	tasks.POST("/batch-postpone", BatchPostponeTasks)
}

func getPlanID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

func getTaskID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

func buildTaskResponse(task Task) gin.H {
	resp := gin.H{
		"id":        task.ID,
		"planId":    task.PlanID,
		"status":    task.Status,
		"createdAt": task.CreatedAt,
	}
	if task.StartedAt != nil {
		resp["startedAt"] = task.StartedAt
	}
	if task.CompletedAt != nil {
		resp["completedAt"] = task.CompletedAt
	}
	if task.Deadline != nil {
		resp["deadline"] = task.Deadline
	}
	if task.Remark != nil {
		resp["remark"] = task.Remark
	}
	return resp
}

func CreateTaskPlan(c *gin.Context) {
	var req struct {
		ParentID  *int         ``
		Name      string       ``
		PlanType  TaskPlanType ``
		CronExpr  *string      ``
		OrderNum  *int         ``
		Priority  *int         ``
		Remark    *string      ``
		Link      *string      ``
		StartedAt *time.Time   ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.PlanType == "" {
		req.PlanType = TaskPlanTypeTodo
	}

	if req.PlanType == TaskPlanTypeTodo && req.StartedAt == nil {
		renv.Error(c, http.StatusBadRequest, "待办类型必须传 startedAt")
		return
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
		PlanType: req.PlanType,
		CronExpr: req.CronExpr,
		OrderNum: orderNum,
		Priority: priority,
		Remark:   req.Remark,
		Link:     req.Link,
	}

	if err := CreateTaskPlanRow(&plan); err != nil {
		renv.Error(c, http.StatusInternalServerError, "创建任务计划失败: " + err.Error())
		return
	}

	resp := gin.H{
		"id":        plan.ID,
		"parentId":  plan.ParentID,
		"name":      plan.Name,
		"status":    plan.Status,
		"planType":  plan.PlanType,
		"cronExpr":  plan.CronExpr,
		"orderNum":  plan.OrderNum,
		"priority":  plan.Priority,
		"remark":    plan.Remark,
		"link":      plan.Link,
		"createdAt": plan.CreatedAt,
		"updatedAt": plan.UpdatedAt,
	}

	if plan.PlanType == TaskPlanTypeTodo {
		firstTask, err := generateTask(&plan, *req.StartedAt, nil)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "生成任务失败: " + err.Error())
			return
		}
		resp["firstTask"] = buildTaskResponse(*firstTask)
	}

	renv.Success(c, resp)
}

func ListTaskPlans(c *gin.Context) {
	plans, err := ListPlans(c.Query("planType"), c.Query("status"))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}
	renv.Success(c, plans)
}

func GetTaskPlanTree(c *gin.Context) {
	plans, err := ListPlanTreeRows()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	if keyword := strings.TrimSpace(c.Query("search")); keyword != "" {
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
		plans = filtered
	}

	tree := buildTaskPlanTree(plans)
	renv.Success(c, tree)
}

func GetTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}
	renv.Success(c, plan)
}

func UpdateTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Name     *string      ``
		ParentID *int         ``
		PlanType TaskPlanType ``
		OrderNum *int         ``
		Priority *int         ``
		Remark   *string      ``
		Link     *string      ``
		CronExpr *string      ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := FindActiveTaskPlan(id); err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	updated := make(map[string]interface{})
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
		plan, _ := GetTaskPlanByID(id)
		renv.Success(c, plan)
		return
	}
	updated["updated_at"] = time.Now()

	if err := UpdateTaskPlanColumns(id, updated); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	plan, err := GetTaskPlanByID(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取失败: " + err.Error())
		return
	}

	renv.Success(c, plan)
}

func StartTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusCreated && plan.Status != TaskPlanStatusStarted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许开启")
		return
	}

	if plan.IsSuspended {
		renv.Error(c, http.StatusBadRequest, "计划已挂起，请先恢复再开启")
		return
	}

	plan.Status = TaskPlanStatusStarted
	plan.UpdatedAt = time.Now()
	if err := SaveTaskPlan(plan); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	activeCount, err := CountActiveTasks(plan.ID)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "检查任务失败: " + err.Error())
		return
	}

	var respTask *Task
	if activeCount == 0 {
		respTask, err = generateTask(plan, time.Now(), nil)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "生成任务失败: " + err.Error())
			return
		}
	}

	resp := map[string]interface{}{
		"id":        plan.ID,
		"name":      plan.Name,
		"status":    plan.Status,
		"planType":  plan.PlanType,
		"updatedAt": plan.UpdatedAt,
	}
	if respTask != nil {
		resp["firstTask"] = buildTaskResponse(*respTask)
	}

	renv.Success(c, resp)
}

func CompleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusStarted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许完成")
		return
	}

	now := time.Now()
	plan.Status = TaskPlanStatusCompleted
	plan.UpdatedAt = now
	if err := CompleteActiveTasksForPlan(id, now); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新任务失败: " + err.Error())
		return
	}

	renv.Success(c, plan)
}

func ArchiveTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.Status != TaskPlanStatusCompleted {
		renv.Error(c, http.StatusBadRequest, "当前状态不允许归档")
		return
	}

	plan.Status = TaskPlanStatusArchived
	plan.UpdatedAt = time.Now()
	if err := SaveTaskPlan(plan); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	renv.Success(c, plan)
}

func DeleteTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	cascade := c.Query("cascade") == "true"

	if _, err := FindActiveTaskPlan(id); err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if !cascade {
		childCount, err := CountChildPlans(id)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "检查子任务失败: " + err.Error())
			return
		}
		activeTaskCount, err := CountActiveTasks(id)
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "检查任务失败: " + err.Error())
			return
		}
		if childCount > 0 {
			renv.Error(c, http.StatusBadRequest, "存在子任务计划，无法删除")
			return
		}
		if activeTaskCount > 0 {
			renv.Error(c, http.StatusBadRequest, "存在未完成的任务，无法删除")
			return
		}
	}

	deletedPlanCount, err := SoftDeletePlanAndTasks(id, cascade)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "删除任务计划失败: " + err.Error())
		return
	}

	resp := gin.H{"message": "删除成功", "cascade": cascade}
	if cascade {
		resp["deletedPlanCount"] = deletedPlanCount
	}
	renv.Success(c, resp)
}

func SuspendTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	if _, err := FindActiveTaskPlan(id); err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := CollectDescendantPlanIDs(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := UpdatePlanColumnsByIDs(ids, map[string]interface{}{
		"is_suspended": true,
		"updated_at":   now,
	}); err != nil {
		renv.Error(c, http.StatusInternalServerError, "挂起失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func ResumeTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	if _, err := FindActiveTaskPlan(id); err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := CollectDescendantPlanIDs(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := UpdatePlanColumnsByIDs(ids, map[string]interface{}{
		"is_suspended": false,
		"updated_at":   now,
	}); err != nil {
		renv.Error(c, http.StatusInternalServerError, "恢复失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func SetPriorityTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Priority int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "优先级必须为整数")
		return
	}
	if req.Priority < 0 {
		renv.Error(c, http.StatusBadRequest, "优先级不能为负数")
		return
	}

	if _, err := FindActiveTaskPlan(id); err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	ids, err := CollectDescendantPlanIDs(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "收集子任务失败: " + err.Error())
		return
	}

	now := time.Now()
	if err := UpdatePlanColumnsByIDs(ids, map[string]interface{}{
		"priority":   req.Priority,
		"updated_at": now,
	}); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新优先级失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func ListPlanTasks(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	tasks, err := ListTasksByPlan(id)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	renv.Success(c, tasks)
}

func GetPendingTasks(c *gin.Context) {
	now := time.Now()
	early := c.Query("early") == "1"

	planID := 0
	if raw := strings.TrimSpace(c.Query("planId")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			planID = v
		}
	}
	name := strings.TrimSpace(c.Query("name"))

	q := pagination.Parse(c)

	result, total, err := QueryPendingTasks(now, early, planID, name,
		c.Query("sort"), c.Query("order"), q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}

	renv.Success(c, pagination.New(result, total, q))
}

func CompleteTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var req struct {
		Rating *int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Rating = nil
	}

	task, err := FindActiveTask(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	if task.Status != TaskStatusActive {
		renv.Error(c, http.StatusBadRequest, "任务已完成")
		return
	}

	plan, err := FindActiveTaskPlan(task.PlanID)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "所属任务计划不存在")
		return
	}

	var rating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		if req.Rating == nil {
			renv.Error(c, http.StatusBadRequest, "interval 类型任务必须传 rating，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy")
			return
		}
		r := FsrsRating(*req.Rating)
		if r < RatingAgain || r > RatingEasy {
			renv.Error(c, http.StatusBadRequest, "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy")
			return
		}
		rating = &r
	}

	nextTask, err := finishTask(task, plan, rating)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "完成任务失败: " + err.Error())
		return
	}

	resp := buildTaskResponse(*task)
	if nextTask != nil {
		resp["nextTask"] = buildTaskResponse(*nextTask)
	}

	renv.Success(c, resp)
}

func PostponeTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	var req struct {
		Days int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}

	task, err := FindActiveTask(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	updates, ok2, reason := computePostponeUpdates(task, req.Days)
	if !ok2 {
		renv.Error(c, http.StatusBadRequest, reason)
		return
	}

	if err := UpdateTaskColumns(task.ID, updates); err != nil {
		renv.Error(c, http.StatusInternalServerError, "延期失败: " + err.Error())
		return
	}

	renv.Success(c, nil)
}

func BatchPostponeTasks(c *gin.Context) {
	var req struct {
		IDs  []int `json:"ids"`
		Days int   `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}
	if len(req.IDs) == 0 {
		renv.Error(c, http.StatusBadRequest, "请选择至少一个任务")
		return
	}

	type ItemResult struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	}
	results := make([]ItemResult, 0, len(req.IDs))
	_ = results
	postponed := 0
	skipped := 0

	for _, id := range req.IDs {
		task, err := FindActiveTask(id)
		if err != nil {
			results = append(results, ItemResult{ID: id, Status: "skipped", Reason: "任务不存在"})
			skipped++
			continue
		}
		updates, ok, reason := computePostponeUpdates(task, req.Days)
		if !ok {
			results = append(results, ItemResult{ID: id, Status: "skipped", Reason: reason})
			skipped++
			continue
		}
		if err := UpdateTaskColumns(id, updates); err != nil {
			renv.Error(c, http.StatusInternalServerError, "延期失败: " + err.Error())
			return
		}
		results = append(results, ItemResult{ID: id, Status: "postponed"})
		postponed++
	}

	_ = postponed
	_ = skipped
	renv.Success(c, nil)
}

func CancelTask(c *gin.Context) {
	id, ok := getTaskID(c)
	if !ok {
		return
	}

	task, err := FindActiveTask(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "任务不存在")
		return
	}

	if task.Status != TaskStatusActive {
		renv.Error(c, http.StatusBadRequest, "任务已完成或已取消")
		return
	}

	plan, err := FindActiveTaskPlan(task.PlanID)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "所属任务计划不存在")
		return
	}

	if plan.PlanType != TaskPlanTypeCron && plan.PlanType != TaskPlanTypeInterval {
		renv.Error(c, http.StatusBadRequest, "仅周期重复任务支持取消")
		return
	}

	now := time.Now()
	task.Status = TaskStatusCancelled
	task.CompletedAt = &now
	if err := SaveTask(task); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}

	resp := buildTaskResponse(*task)

	nextTask, err := generateTask(plan, now, nil)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "生成下一条任务失败: " + err.Error())
		return
	}
	resp["nextTask"] = buildTaskResponse(*nextTask)

	renv.Success(c, resp)
}
