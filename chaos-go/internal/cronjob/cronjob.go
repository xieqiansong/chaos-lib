// Package cronjob 是「定时任务」模块：cron 调度 + 动作执行 + 运行日志。
//
// 与标准数据（internal/standarddata）同构，遵循业务模块脚手架基线：
// 纯 CRUD 交给通用 crud（反射生成五路由，免写标准 handler），本业务特有的
// 扩展能力（启停 / 立即执行 / 运行历史 / cron 预览）作为自定义子路由由本包自实现并挂载；
// 字段校验与调度器同步则通过 crud 的写方向钩子注入（钩子失败即回滚，不落库也不进调度器）。
package cronjob

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/config"
	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
)

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 标准 CRUD 由通用 crud 反射生成，扩展能力挂在同一前缀下，routes.go 只写一行 cronjob.Register(api)。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "cronJob", &CronJob{}, crud.Opts{
		Searchable:  []string{"name", "cron_expr", "action_type"},
		Sortable:    []string{"id", "name", "enabled", "created_at"},
		ToResponse:  toResponse,
		AfterCreate: afterSave,
		AfterUpdate: afterSave,
		AfterDelete: func(row any) error {
			RemoveJob(row.(*CronJob).ID)
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

// ── 基线回调：校验 + 调度同步 ──

// afterSave 在 create / update 事务内、提交前执行：先校验（失败即回滚，
// 既不写库也不进调度器），通过后再把最新状态同步进 cron 调度器。
func afterSave(row any) error {
	job, ok := row.(*CronJob)
	if !ok {
		return nil
	}
	if err := validateJob(job); err != nil {
		return err
	}
	SyncJob(job)
	return nil
}

// toResponse 把查询结果整批转换为响应形态（模型 + 派生的下次执行时间）。
func toResponse(rows any) any {
	list, ok := rows.([]*CronJob)
	if !ok {
		return rows
	}
	out := make([]CronJobView, 0, len(list))
	for _, job := range list {
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

// ── 校验 ──

// validateJob 校验任务字段：名称 / cron 表达式 / 超时 / 动作配置。
func validateJob(job *CronJob) error {
	if strings.TrimSpace(job.Name) == "" {
		return errors.New("任务名称不能为空")
	}
	if strings.TrimSpace(job.CronExpr) == "" {
		return errors.New("cron 表达式不能为空")
	}
	if _, err := cronParser.Parse(job.CronExpr); err != nil {
		return errors.New("无效的 cron 表达式: " + err.Error())
	}
	if job.TimeoutSec < 0 {
		return errors.New("超时时间不能为负数")
	}
	return validateAction(job)
}

// validateAction 按动作类型校验 ActionConfig（JSON 文本）内容是否完整。
func validateAction(job *CronJob) error {
	switch job.ActionType {
	case ActionTypeHTTP:
		var act HTTPAction
		if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
			return errors.New("解析 HTTP 动作配置失败: " + err.Error())
		}
		if strings.TrimSpace(act.URL) == "" {
			return errors.New("HTTP 动作的 URL 不能为空")
		}
		return nil
	case ActionTypeShell:
		var act ShellAction
		if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
			return errors.New("解析命令动作配置失败: " + err.Error())
		}
		if strings.TrimSpace(act.Command) == "" {
			return errors.New("命令动作的命令不能为空")
		}
		return nil
	default:
		return errors.New("未知动作类型: " + string(job.ActionType))
	}
}

// ── 扩展能力（标准 CRUD 之外的自定义子路由）──

// loadJob 解析路径 id 并读取未删除的任务；失败时已写好响应，返回 false。
func loadJob(c *gin.Context) (CronJob, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return CronJob{}, false
	}
	var job CronJob
	if err := config.GetDB().Where("is_deleted = ?", false).First(&job, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job, ok := loadJob(c)
	if !ok {
		return
	}
	if err := config.GetDB().Model(&CronJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"enabled":    req.Status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "状态更新失败: " + err.Error()})
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
	c.JSON(http.StatusOK, gin.H{"message": "状态更新成功", "data": &view})
}

// run 立即手动触发一次（POST /cronJob/:id/run），返回本次运行记录。
func run(c *gin.Context) {
	job, ok := loadJob(c)
	if !ok {
		return
	}
	ExecuteJob(&job)
	var latest CronJobRun
	if err := config.GetDB().Where("job_id = ?", job.ID).Order("started_at DESC").First(&latest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, &latest)
}

// runs 查询某任务的运行历史（GET /cronJob/:id/runs，分页）。
func runs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}
	q := pagination.Parse(c)
	var list []CronJobRun
	base := config.GetDB().Model(&CronJobRun{}).Where("job_id = ?", id).Order("started_at DESC")
	total, err := pagination.Paginate(base, &list, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, pagination.New(list, total, q))
}

// preview 校验 cron 表达式并返回未来若干次触发时间（POST /cronJob/preview，body {cronExpr,count}）。
func preview(c *gin.Context) {
	var req struct {
		CronExpr string `json:"cronExpr"`
		Count    int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.CronExpr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cron 表达式不能为空"})
		return
	}
	if req.Count <= 0 || req.Count > 20 {
		req.Count = 5
	}
	next, err := NextRuns(req.CronExpr, req.Count)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error(), "nextRuns": []time.Time{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "nextRuns": next})
}

// ── 默认数据 ──

// SeedDefaults 在库内无任何定时任务时，写入由原系统内置周期任务转换而来的默认任务，
// 保证「改为 API + 由本模块触发」后原有行为不中断。
func SeedDefaults() {
	db := config.GetDB()
	var count int64
	if err := db.Model(&CronJob{}).Where("is_deleted = ?", false).Count(&count).Error; err != nil {
		slog.Error("统计定时任务失败", "err", err)
		return
	}
	if count > 0 {
		return
	}
	defaults := []CronJob{
		{Name: "任务计划扫描", CronExpr: "*/1 * * * *", ActionType: ActionTypeHTTP, ActionConfig: mustJSON(HTTPAction{Method: http.MethodPost, URL: "/api/systemJobs/sweep"}), Enabled: true, TimeoutSec: 30},
		{Name: "端口转发自愈", CronExpr: "*/5 * * * *", ActionType: ActionTypeHTTP, ActionConfig: mustJSON(HTTPAction{Method: http.MethodPost, URL: "/api/systemJobs/portForwardSelfHeal"}), Enabled: true, TimeoutSec: 60},
		{Name: "STUN 规则同步", CronExpr: "*/1 * * * *", ActionType: ActionTypeHTTP, ActionConfig: mustJSON(HTTPAction{Method: http.MethodPost, URL: "/api/systemJobs/stunRuleSync"}), Enabled: true, TimeoutSec: 30},
		{Name: "STUN 端口转发同步", CronExpr: "*/30 * * * * *", ActionType: ActionTypeHTTP, ActionConfig: mustJSON(HTTPAction{Method: http.MethodPost, URL: "/api/systemJobs/stunPortForwardSync"}), Enabled: true, TimeoutSec: 30},
	}
	if err := db.Create(&defaults).Error; err != nil {
		slog.Error("写入默认定时任务失败", "err", err)
		return
	}
	slog.Info("已写入默认定时任务", "count", len(defaults))
}
