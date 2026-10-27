package cronjob

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

var jobErrRules = []httpx.ErrRule{
	{Err: ErrJobNotFound, Status: http.StatusNotFound, Msg: "定时任务不存在"},
}

var cronJobOpts = crud.Opts[CronJob]{
	Searchable:  []string{"name", "cron_expr", "action_type"},
	Sortable:    []string{"id", "name", "enabled", "created_at"},
	ToResponse:  CronJobViews,
	AfterCreate: OnSaved,
	AfterUpdate: OnSaved,
	AfterDelete: RemoveJobOnDelete,
}
var cronJobToggle = &crud.ToggleOpts{

	Setter:   func(id int, status bool) (any, error) { return SetEnabled(id, status) },
	ErrRules: jobErrRules,
}

type runReq struct {
	ID int `json:"id"`
}
type runsReq struct {
	ID       int `json:"id"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}
type previewReq struct {
	CronExpr string `json:"cronExpr"`
	Count    int    `json:"count"`
}

func RegisterV1(rg *web.RouterGroup) {
	g := crud.RegisterActions[CronJob](rg, "cron-jobs", cronJobOpts, cronJobToggle)
	g.POST("/run", runV1)
	g.POST("/runs", runsV1)
	g.POST("/preview", previewV1)
	g.POST("/builtinList", builtinListV1)
}

func builtinListV1(c *web.Context) {
	renv.Success(c, BuiltinTasks())
}

func run(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doRun(c, id)
}

func runV1(c *web.Context) {
	var req runReq
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doRun(c, req.ID)
}

func doRun(c *web.Context, id int) {
	latest, err := ExecuteNow(id)
	if err != nil {
		httpx.MapError(c, err, jobErrRules)
		return
	}
	renv.Success(c, latest)
}

func runs(c *web.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doRuns(c, id, pagination.Parse(c))
}

func runsV1(c *web.Context) {
	var req runsReq
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	q := pagination.Query{Page: req.Page, PageSize: req.PageSize}
	if q.Page < 1 {
		q.Page = pagination.DefaultPage
	}
	if q.PageSize < 1 || q.PageSize > pagination.MaxPageSize {
		q.PageSize = pagination.DefaultPageSize
	}
	doRuns(c, req.ID, q)
}

func doRuns(c *web.Context, id int, q pagination.Query) {
	list, total, err := FindRuns(id, q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(list, total, q))
}

func preview(c *web.Context) {
	var req previewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doPreview(c, req)
}

func previewV1(c *web.Context) {
	var req previewReq
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doPreview(c, req)
}

func doPreview(c *web.Context, req previewReq) {
	if req.CronExpr == "" {
		renv.Error(c, http.StatusBadRequest, "cron 表达式不能为空")
		return
	}
	next, err := PreviewRuns(req.CronExpr, req.Count)
	if err != nil {
		renv.Error(c, http.StatusOK, err.Error())
		return
	}
	renv.Success(c, map[string]any{"valid": true, "nextRuns": next})
}
