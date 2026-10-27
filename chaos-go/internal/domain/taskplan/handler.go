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

	"chaos-go/internal/framework/web"
)

func CreateTaskPlan(c *web.Context) {
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

func ListTaskPlans(c *web.Context) {
	plans, err := ListPlans(c.Query("planType"), c.Query("status"))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, plans)
}

func GetTaskPlanTree(c *web.Context) {
	tree, err := PlanTree(c.Query("search"))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, tree)
}

func GetTaskPlan(c *web.Context) {
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

func UpdateTaskPlan(c *web.Context) {
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

func StartTaskPlan(c *web.Context) {
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

func CompleteTaskPlan(c *web.Context) {
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

func ArchiveTaskPlan(c *web.Context) {
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

func DeleteTaskPlan(c *web.Context) {
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

func SuspendTaskPlan(c *web.Context) {
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

func ResumeTaskPlan(c *web.Context) {
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

func SetPriorityTaskPlan(c *web.Context) {
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

func ListPlanTasks(c *web.Context) {
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

func GetPendingTasks(c *web.Context) {
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

func CompleteTask(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Rating *int
	}

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

func PostponeTask(c *web.Context) {
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

func BatchPostponeTasks(c *web.Context) {
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

func CancelTask(c *web.Context) {
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

var errRules = []httpx.ErrRule{
	{Err: ErrPlanNotFound, Status: http.StatusNotFound, Msg: "任务计划不存在"},
	{Err: ErrTaskNotFound, Status: http.StatusNotFound, Msg: "任务不存在"},
	{Err: ErrInvalidState, Status: http.StatusBadRequest},
}
