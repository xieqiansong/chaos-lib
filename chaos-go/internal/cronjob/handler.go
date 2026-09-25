package cronjob

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"chaos-go/config"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
)

func getJobID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return 0, false
	}
	return id, true
}

// ListCronJobs 列出定时任务，并附下次执行时间。
func ListCronJobs(c *gin.Context) {
	var jobs []CronJob
	if err := config.GetDB().Where("is_deleted = ?", false).Order("id ASC").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	type item struct {
		CronJob
		NextRun *time.Time `json:"nextRun"`
	}
	items := make([]item, 0, len(jobs))
	for _, j := range jobs {
		it := item{CronJob: j}
		if j.Enabled {
			if runs, err := NextRuns(j.CronExpr, 1); err == nil && len(runs) > 0 {
				t := runs[0]
				it.NextRun = &t
			}
		}
		items = append(items, it)
	}
	c.JSON(http.StatusOK, items)
}

type createReq struct {
	Name         string     `json:"name"`
	CronExpr     string     `json:"cronExpr"`
	ActionType   ActionType `json:"actionType"`
	ActionConfig string     `json:"actionConfig"`
	TimeoutSec   *int       `json:"timeoutSec"`
	Enabled      *bool      `json:"enabled"`
}

func validateCron(expr string) error {
	_, err := cronParser.Parse(expr)
	return err
}

// CreateCronJob 新建定时任务。
func CreateCronJob(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务名称不能为空"})
		return
	}
	if req.CronExpr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cron 表达式不能为空"})
		return
	}
	if err := validateCron(req.CronExpr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 cron 表达式: " + err.Error()})
		return
	}
	if req.ActionType == "" {
		req.ActionType = ActionTypeHTTP
	}
	job := CronJob{
		Name:         req.Name,
		CronExpr:     req.CronExpr,
		ActionType:   req.ActionType,
		ActionConfig: req.ActionConfig,
		Enabled:      true,
		TimeoutSec:   30,
	}
	if req.TimeoutSec != nil {
		job.TimeoutSec = *req.TimeoutSec
	}
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	}
	if err := config.GetDB().Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	SyncJob(&job)
	c.JSON(http.StatusCreated, job)
}

// GetCronJob 读取单条定时任务。
func GetCronJob(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	var job CronJob
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
		return
	}
	c.JSON(http.StatusOK, job)
}

type updateReq struct {
	Name         *string     `json:"name"`
	CronExpr     *string     `json:"cronExpr"`
	ActionType   *ActionType `json:"actionType"`
	ActionConfig *string     `json:"actionConfig"`
	TimeoutSec   *int        `json:"timeoutSec"`
	Enabled      *bool       `json:"enabled"`
}

// UpdateCronJob 修改定时任务。
func UpdateCronJob(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var job CronJob
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.CronExpr != nil {
		if err := validateCron(*req.CronExpr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 cron 表达式: " + err.Error()})
			return
		}
		updates["cron_expr"] = *req.CronExpr
	}
	if req.ActionType != nil {
		updates["action_type"] = *req.ActionType
	}
	if req.ActionConfig != nil {
		updates["action_config"] = *req.ActionConfig
	}
	if req.TimeoutSec != nil {
		updates["timeout_sec"] = *req.TimeoutSec
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		c.JSON(http.StatusOK, job)
		return
	}
	updates["updated_at"] = time.Now()
	if err := config.GetDB().Model(&job).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	config.GetDB().First(&job, id)
	SyncJob(&job)
	c.JSON(http.StatusOK, job)
}

// DeleteCronJob 软删除定时任务。
func DeleteCronJob(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	var job CronJob
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
		return
	}
	if err := config.GetDB().Model(&job).Updates(map[string]interface{}{
		"is_deleted": true,
		"enabled":    false,
		"updated_at": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	RemoveJob(id)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ToggleCronJob 启停定时任务。
func ToggleCronJob(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	var job CronJob
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
		return
	}
	job.Enabled = !job.Enabled
	if err := config.GetDB().Model(&job).Updates(map[string]interface{}{
		"enabled":    job.Enabled,
		"updated_at": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	SyncJob(&job)
	c.JSON(http.StatusOK, gin.H{"id": job.ID, "enabled": job.Enabled})
}

// RunCronJob 立即手动触发一次，并返回最新运行记录。
func RunCronJob(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	var job CronJob
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务不存在"})
		return
	}
	ExecuteJob(&job)
	var run CronJobRun
	if err := config.GetDB().Where("job_id = ?", id).Order("started_at DESC").First(&run).Error; err == nil {
		c.JSON(http.StatusOK, run)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已触发"})
}

// ListCronJobRuns 查询某任务的运行历史（分页）。
func ListCronJobRuns(c *gin.Context) {
	id, ok := getJobID(c)
	if !ok {
		return
	}
	q := pagination.Parse(c)
	var runs []CronJobRun
	base := config.GetDB().Model(&CronJobRun{}).Where("job_id = ?", id).Order("started_at DESC")
	total, err := pagination.Paginate(base, &runs, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, pagination.New(runs, total, q))
}

type previewReq struct {
	CronExpr string `json:"cronExpr"`
	Count    int    `json:"count"`
}

// PreviewCron 校验 cron 表达式并返回未来若干次触发时间（供前端表单实时预览）。
func PreviewCron(c *gin.Context) {
	var req previewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Count <= 0 || req.Count > 20 {
		req.Count = 5
	}
	if req.CronExpr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cron 表达式不能为空"})
		return
	}
	runs, err := NextRuns(req.CronExpr, req.Count)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error(), "nextRuns": []time.Time{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "nextRuns": runs})
}

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
		{Name: "STUN 端口转发同步", CronExpr: "*/1 * * * *", ActionType: ActionTypeHTTP, ActionConfig: mustJSON(HTTPAction{Method: http.MethodPost, URL: "/api/systemJobs/stunPortForwardSync"}), Enabled: true, TimeoutSec: 30},
	}
	if err := db.Create(&defaults).Error; err != nil {
		slog.Error("写入默认定时任务失败", "err", err)
		return
	}
	slog.Info("已写入默认定时任务", "count", len(defaults))
}
