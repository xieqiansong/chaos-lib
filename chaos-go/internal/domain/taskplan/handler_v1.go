package taskplan

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func metaToQuery(c *web.Context) {
	envelope.MetaToQuery(c)
}

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/task-plans")

	g.POST("/get", GetTaskPlan)
	g.POST("/start", StartTaskPlan)
	g.POST("/complete", CompleteTaskPlan)
	g.POST("/archive", ArchiveTaskPlan)
	g.POST("/suspend", SuspendTaskPlan)
	g.POST("/resume", ResumeTaskPlan)
	g.POST("/delete", func(c *web.Context) { metaToQuery(c); DeleteTaskPlan(c) })
	g.POST("/listTasks", ListPlanTasks)
	g.POST("/getRaw", GetTaskPlanRaw)

	g.POST("/list", func(c *web.Context) { metaToQuery(c); ListTaskPlans(c) })
	g.POST("/tree", func(c *web.Context) { metaToQuery(c); GetTaskPlanTree(c) })

	tasks := g.Group("/tasks")
	tasks.POST("/pending", func(c *web.Context) { metaToQuery(c); GetPendingTasks(c) })
	tasks.POST("/dailyStats", func(c *web.Context) { metaToQuery(c); GetTaskDailyStats(c) })
	tasks.POST("/activeStats", func(c *web.Context) { metaToQuery(c); GetTaskActiveStats(c) })
	tasks.POST("/contributionStats", func(c *web.Context) { metaToQuery(c); GetTaskContributionStats(c) })
	tasks.POST("/cancel", CancelTask)
	tasks.POST("/complete", taskCompleteV1)
	tasks.POST("/postpone", taskPostponeV1)
	tasks.POST("/batchPostpone", taskBatchPostponeV1)

	ai := g.Group("/ai")
	ai.POST("/reviewScore", aiReviewScoreV1)

	g.POST("/create", planCreateV1)
	g.POST("/update", planUpdateV1)
	g.POST("/setPriority", planSetPriorityV1)
	g.POST("/review", planReviewV1)
}

func planCreateV1(c *web.Context) {
	var req CreatePlanRequest
	if err := envelope.Bind(c, &req); err != nil {
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

func planUpdateV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req UpdatePlanRequest
	if err := envelope.Bind(c, &req); err != nil {
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

func planSetPriorityV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Priority int `json:"priority"`
	}
	if err := envelope.Bind(c, &req); err != nil {
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

func planReviewV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req ReviewRequest
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := ReviewPlan(id, req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

func taskCompleteV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Rating *int `json:"rating"`
	}
	_ = envelope.Bind(c, &req)
	resp, err := CompleteTaskByID(id, req.Rating)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
}

func taskPostponeV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req struct {
		Days int `json:"days"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.Days <= 0 {
		renv.Error(c, http.StatusBadRequest, "延期天数必须为正整数")
		return
	}
	if err := PostponeTaskByID(id, req.Days); err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nil)
}

func taskBatchPostponeV1(c *web.Context) {
	var req struct {
		IDs  []int `json:"ids"`
		Days int   `json:"days"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.Days <= 0 {
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

func aiReviewScoreV1(c *web.Context) {
	var req struct {
		Original string `json:"original"`
		Answer   string `json:"answer"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	score, err := ScoreReview(req.Original, req.Answer)
	if err != nil {
		if errors.Is(err, ErrInvalidState) {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		renv.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	renv.Success(c, score)
}
