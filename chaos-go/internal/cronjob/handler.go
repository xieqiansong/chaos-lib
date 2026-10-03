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
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("cron-jobs", Register)
}

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 标准 CRUD 由通用 crud 反射生成，扩展能力挂在同一前缀下，routes.go 只写一行 cronjob.Register(api)。
func Register(rg *gin.RouterGroup) {
	crud.Register[CronJob](rg, "cron-jobs", crud.Opts[CronJob]{
		Searchable:  []string{"name", "cron_expr", "action_type"},
		Sortable:    []string{"id", "name", "enabled", "created_at"},
		ToResponse:  CronJobViews,
		AfterCreate: OnSaved,
		AfterUpdate: OnSaved,
		AfterDelete: RemoveJobOnDelete,
	})
	g := rg.Group("/cron-jobs")
	{
		g.PATCH("/:id/status", status) // 启停（带调度副作用）
		g.POST("/:id/run", run)        // 立即执行一次
		g.GET("/:id/runs", runs)       // 运行历史（分页）
		g.POST("/preview", preview)    // cron 表达式校验 + 触发时间预览
	}
}

// status 启停定时任务（PATCH /cronJob/:id/status，body {status:bool}）。
// 启用与否直接决定调度器是否挂载该任务，故不走通用 PATCH，改库后同步调度器。
func status(c *gin.Context) {
	var req struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	view, err := SetEnabled(id, req.Status)
	if err != nil {
		writeJobError(c, err)
		return
	}
	renv.Success(c, view)
}

// run 立即手动触发一次（POST /cronJob/:id/run），返回本次运行记录。
func run(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	latest, err := ExecuteNow(id)
	if err != nil {
		writeJobError(c, err)
		return
	}
	renv.Success(c, latest)
}

// runs 查询某任务的运行历史（GET /cronJob/:id/runs，分页）。
func runs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	q := pagination.Parse(c)
	list, total, err := FindRuns(id, q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(list, total, q))
}

// preview 校验 cron 表达式并返回未来若干次触发时间（POST /cronJob/preview，body {cronExpr,count}）。
func preview(c *gin.Context) {
	var req struct {
		CronExpr string `json:"cronExpr"`
		Count    int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
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

// writeJobError 把 service / repository 返回的领域错误映射为 HTTP 状态码。
func writeJobError(c *gin.Context, err error) {
	if errors.Is(err, ErrJobNotFound) {
		renv.Error(c, http.StatusNotFound, "定时任务不存在")
		return
	}
	renv.Error(c, http.StatusInternalServerError, err.Error())
}
