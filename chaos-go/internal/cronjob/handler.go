// Package cronjob 是「定时任务」模块：cron 调度 + 动作执行 + 运行日志。
//
// 与标准数据（internal/standarddata）同构，遵循业务模块脚手架基线：
// 纯 CRUD 交给通用 crud（反射生成五路由，免写标准 handler），本业务特有的
// 扩展能力（启停 / 立即执行 / 运行历史 / cron 预览）作为自定义子路由由本包自实现并挂载；
// 字段校验与调度器同步则通过 crud 的写方向钩子注入（钩子失败即回滚，不落库也不进调度器）。
package cronjob

import (
	"net/http"
	"strconv"

	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"

	"github.com/gin-gonic/gin"
)

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 标准 CRUD 由通用 crud 反射生成，扩展能力挂在同一前缀下，routes.go 只写一行 cronjob.Register(api)。
func Register(rg *gin.RouterGroup) {
	crud.Register[CronJob](rg, "cronJob", crud.Opts[CronJob]{
		Searchable:  []string{"name", "cron_expr", "action_type"},
		Sortable:    []string{"id", "name", "enabled", "created_at"},
		ToResponse:  toResponse,
		AfterCreate: afterSave,
		AfterUpdate: afterSave,
		AfterDelete: func(row *CronJob) error {
			RemoveJob(row.ID)
			return nil
		},
	})
	g := rg.Group("/cronJob")
	{
		g.PATCH("/:id/status", status) // 启停（带调度副作用）
		g.POST("/:id/run", run)        // 立即执行一次
		g.GET("/:id/runs", runs)       // 运行历史（分页）
		g.POST("/preview", preview)    // cron 表达式校验 + 触发时间预览
	}
}

// toResponse 把查询结果整批转换为响应形态（模型 + 派生的下次执行时间）。
func toResponse(rows []*CronJob) any {
	out := make([]CronJobView, 0, len(rows))
	for _, job := range rows {
		view := CronJobView{CronJob: *job}
		if job.Enabled && !job.IsDeleted {
			if next, err := NextRuns(job.CronExpr, 1); err == nil && len(next) > 0 {
				t := next[0]
				view.NextRun = &t
			}
		}
		out = append(out, view)
	}
	return out
}

// loadJob 解析路径 id 并读取未删除的任务；失败时已写好响应，返回 false。
func loadJob(c *gin.Context) (CronJob, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return CronJob{}, false
	}
	job, err := FindJobByID(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "定时任务不存在")
		return CronJob{}, false
	}
	return job, true
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
	job, ok := loadJob(c)
	if !ok {
		return
	}
	if err := UpdateJobEnabled(job.ID, req.Status); err != nil {
		renv.Error(c, http.StatusInternalServerError, "状态更新失败: "+err.Error())
		return
	}
	job.Enabled = req.Status
	SyncJob(&job)
	view := CronJobView{CronJob: job}
	if job.Enabled {
		if next, err := NextRuns(job.CronExpr, 1); err == nil && len(next) > 0 {
			t := next[0]
			view.NextRun = &t
		}
	}
	renv.Success(c, &view)
}

// run 立即手动触发一次（POST /cronJob/:id/run），返回本次运行记录。
func run(c *gin.Context) {
	job, ok := loadJob(c)
	if !ok {
		return
	}
	ExecuteJob(&job)
	latest, err := FindLatestRun(job.ID)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取运行记录失败: "+err.Error())
		return
	}
	renv.Success(c, &latest)
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
	if req.Count <= 0 || req.Count > 20 {
		req.Count = 5
	}
	next, err := NextRuns(req.CronExpr, req.Count)
	if err != nil {
		renv.Error(c, http.StatusOK, err.Error())
		return
	}
	renv.Success(c, gin.H{"valid": true, "nextRuns": next})
}
