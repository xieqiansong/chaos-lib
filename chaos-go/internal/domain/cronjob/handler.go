// Package cronjob 是「定时任务」模块：cron 调度 + 动作执行 + 运行日志。
//
// 与标准数据（internal/standarddata）同构，遵循业务模块脚手架基线：
// 纯 CRUD 交给通用 crud（反射生成五路由，免写标准 handler），本业务特有的
// 扩展能力（启停 / 立即执行 / 运行历史 / cron 预览）作为自定义子路由由本包自实现并挂载；
// 字段校验与调度器同步则通过 crud 的写方向钩子注入（钩子失败即回滚，不落库也不进调度器）。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：实体 + 动作配置结构
//   - repository.go：数据访问
//   - scheduler.go：cron 调度运行时
//   - executor.go：动作执行（HTTP / Shell）
//   - seed.go：默认任务
//   - service.go：校验 + 用例编排
//   - dto.go：响应契约
//   - handler.go：参数解析 + 状态码映射 + 路由注册
package cronjob

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.RegisterV1("cron-jobs", RegisterV1)
}

// jobErrRules 领域错误 → HTTP 状态码映射表，取代原先内联在 handler 里的 writeJobError。
var jobErrRules = []httpx.ErrRule{
	{Err: ErrJobNotFound, Status: http.StatusNotFound, Msg: "定时任务不存在"},
}

// cronJobOpts / cronJobToggle 标准 CRUD 与启停选项，存量 /api 与 v1 动作路由共用。
var cronJobOpts = crud.Opts[CronJob]{
	Searchable:  []string{"name", "cron_expr", "action_type"},
	Sortable:    []string{"id", "name", "enabled", "created_at"},
	ToResponse:  CronJobViews,
	AfterCreate: OnSaved,
	AfterUpdate: OnSaved,
	AfterDelete: RemoveJobOnDelete,
}
var cronJobToggle = &crud.ToggleOpts{
	// 启用与否直接决定调度器是否挂载该任务，故启停带副作用，不走通用 PATCH。
	Setter:   func(id int, status bool) (any, error) { return SetEnabled(id, status) },
	ErrRules: jobErrRules,
}

// request 结构：自定义动作的入参，存量 /api 与 v1 共用字段语义。
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

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 标准 CRUD + status（启停）由通用 crud 生成；自定义动作 run / runs / preview 一并迁为 POST 动作。
func RegisterV1(rg *gin.RouterGroup) {
	g := crud.RegisterActions[CronJob](rg, "cron-jobs", cronJobOpts, cronJobToggle)
	g.POST("/run", runV1)
	g.POST("/runs", runsV1)
	g.POST("/preview", previewV1)
}

// ── 立即执行 ──────────────────────────────────────────────────────

// run 立即手动触发一次（POST /cron-jobs/:id/run），返回本次运行记录。
func run(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doRun(c, id)
}

// runV1 是 run 的「POST + Action」版（POST /api/v1/cron-jobs/run，body {id}）。
func runV1(c *gin.Context) {
	var req runReq
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doRun(c, req.ID)
}

func doRun(c *gin.Context, id int) {
	latest, err := ExecuteNow(id)
	if err != nil {
		httpx.MapError(c, err, jobErrRules)
		return
	}
	renv.Success(c, latest)
}

// ── 运行历史 ──────────────────────────────────────────────────────

// runs 查询某任务的运行历史（GET /cron-jobs/:id/runs，分页）。
func runs(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doRuns(c, id, pagination.Parse(c))
}

// runsV1 是 runs 的「POST + Action」版（POST /api/v1/cron-jobs/runs，body {id,page,pageSize}）。
func runsV1(c *gin.Context) {
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

func doRuns(c *gin.Context, id int, q pagination.Query) {
	list, total, err := FindRuns(id, q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(list, total, q))
}

// ── cron 预览 ─────────────────────────────────────────────────────

// preview 校验 cron 表达式并返回未来若干次触发时间（POST /cron-jobs/preview，body {cronExpr,count}）。
func preview(c *gin.Context) {
	var req previewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doPreview(c, req)
}

// previewV1 是 preview 的「POST + Action」版（POST /api/v1/cron-jobs/preview，信封 data 同结构）。
func previewV1(c *gin.Context) {
	var req previewReq
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doPreview(c, req)
}

func doPreview(c *gin.Context, req previewReq) {
	if req.CronExpr == "" {
		renv.Error(c, http.StatusBadRequest, "cron 表达式不能为空")
		return
	}
	next, err := PreviewRuns(req.CronExpr, req.Count)
	if err != nil {
		renv.Error(c, http.StatusOK, err.Error())
		return
	}
	renv.Success(c, gin.H{"valid": true, "nextRuns": next})
}
