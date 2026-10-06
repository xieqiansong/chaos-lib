package taskplan

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("task-plans", Register)
}

// Register 把任务计划与待办任务全部路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 注意：本模块未走通用 crud，由专用 handler 直接接线，以承载 FSRS / 调度 / 树形等定制行为。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/task-plans")
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

// ── 计划 ────────────────────────────────────────────────────────

// CreateTaskPlan 创建任务计划（待办类型须带 startedAt，并生成首条任务）。
func CreateTaskPlan(c *gin.Context) {
	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := CreatePlan(req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// ListTaskPlans 按类型 / 状态列出计划。
func ListTaskPlans(c *gin.Context) {
	plans, err := ListPlans(c.Query("planType"), c.Query("status"))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, plans)
}

// GetTaskPlanTree 返回计划树；?search= 按名称 / 备注过滤并保留祖先链。
func GetTaskPlanTree(c *gin.Context) {
	tree, err := PlanTree(c.Query("search"))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, tree)
}

// GetTaskPlan 返回单个计划。
func GetTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	plan, err := FindActiveTaskPlan(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, plan)
}

// UpdateTaskPlan 按字段更新计划。
func UpdateTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := UpdatePlan(id, req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, plan)
}

// StartTaskPlan 开启计划（必要时生成首条任务）。
func StartTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	resp, err := StartPlan(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// CompleteTaskPlan 完成计划并结束其进行中的任务。
func CompleteTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	plan, err := CompletePlan(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, plan)
}

// ArchiveTaskPlan 归档计划。
func ArchiveTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	plan, err := ArchivePlan(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, plan)
}

// DeleteTaskPlan 删除计划；?cascade=true 时连同子孙计划与任务一起删除。
func DeleteTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	resp, err := DeletePlan(id, c.Query("cascade") == "true")
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// SuspendTaskPlan 挂起计划及其子孙。
func SuspendTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	if err := SuspendPlans(id); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

// ResumeTaskPlan 恢复计划及其子孙。
func ResumeTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	if err := ResumePlans(id); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

// SetPriorityTaskPlan 设置计划及其子孙的优先级。
func SetPriorityTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Priority int
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "优先级必须为整数")
		return
	}
	if req.Priority < 0 {
		renv.Error(c, http.StatusBadRequest, "优先级不能为负数")
		return
	}
	if err := SetPlanPriority(id, req.Priority); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

// ListPlanTasks 列出某计划下的任务。
func ListPlanTasks(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	tasks, err := PlanTasks(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, tasks)
}

// ── 任务 ────────────────────────────────────────────────────────

// GetPendingTasks 待处理任务列表（分页）。
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

	result, total, err := QueryPendingTasks(now, early, planID, name, c.Query("sort"), c.Query("order"), q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(result, total, q))
}

// CompleteTask 完成任务（interval 类型须带 rating）。
func CompleteTask(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Rating *int
	}
	// 评分可选：非 interval 类型不传也允许，故绑定失败按「无评分」处理。
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Rating = nil
	}
	resp, err := CompleteTaskByID(id, req.Rating)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// PostponeTask 单条任务延期（body {days}）。
func PostponeTask(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Days int
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}
	if err := PostponeTaskByID(id, req.Days); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

// BatchPostponeTasks 批量延期（body {ids, days}），返回逐项结果与汇总。
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
	resp, err := PostponeTasks(req.IDs, req.Days)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// CancelTask 取消任务（仅 cron / interval 支持），并生成下一条。
func CancelTask(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	resp, err := CancelTaskByID(id)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

// ── 辅助 ────────────────────────────────────────────────────────

// errRules 领域错误 → HTTP 状态码映射表，取代原先内联在 handler 里的 writeError。
var errRules = []httpx.ErrRule{
	{Err: ErrPlanNotFound, Status: http.StatusNotFound, Msg: "任务计划不存在"},
	{Err: ErrTaskNotFound, Status: http.StatusNotFound, Msg: "任务不存在"},
	{Err: ErrInvalidState, Status: http.StatusBadRequest},
}
