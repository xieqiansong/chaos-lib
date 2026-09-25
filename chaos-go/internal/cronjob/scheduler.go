package cronjob

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"chaos-go/config"

	"github.com/robfig/cron/v3"
)

var (
	// SecondOptional：同时支持 5 字段（分 时 日 月 周）与 6 字段（秒 分 时 日 月 周），
	// 即 cron 表达式可精确到秒级（如 "*/30 * * * * *" 表示每 30 秒）。
	cronParser   = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronInstance *cron.Cron
	cronMu       sync.Mutex
	entryIDs     = map[int]cron.EntryID{}
)

// Start 加载所有启用的定时任务并启动调度器。
func Start() {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance != nil {
		cronInstance.Stop()
	}
	cronInstance = cron.New(cron.WithParser(cronParser))
	var jobs []CronJob
	if err := config.GetDB().Where("enabled = ? AND is_deleted = ?", true, false).Find(&jobs).Error; err != nil {
		slog.Error("加载定时任务失败", "err", err)
	}
	for i := range jobs {
		addEntry(&jobs[i])
	}
	cronInstance.Start()
	slog.Info("cronjob 调度器启动", "jobs", len(jobs))
}

// Stop 停止调度器（优雅退出 / 测试用）。
func Stop() {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance != nil {
		cronInstance.Stop()
		cronInstance = nil
	}
	entryIDs = map[int]cron.EntryID{}
}

// SyncJob 在任务变更后重排调度（新增 / 修改 / 启停）。
func SyncJob(job *CronJob) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if cronInstance == nil {
		return
	}
	if old, ok := entryIDs[job.ID]; ok {
		cronInstance.Remove(old)
		delete(entryIDs, job.ID)
	}
	if job.Enabled && !job.IsDeleted {
		addEntry(job)
	}
}

// RemoveJob 从调度器移除（删除时调用）。
func RemoveJob(id int) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if old, ok := entryIDs[id]; ok {
		cronInstance.Remove(old)
		delete(entryIDs, id)
	}
}

func addEntry(job *CronJob) {
	if cronInstance == nil {
		return
	}
	schedule, err := cronParser.Parse(job.CronExpr)
	if err != nil {
		slog.Error("cron 表达式无效，跳过调度", "jobId", job.ID, "expr", job.CronExpr, "err", err)
		return
	}
	j := *job
	id := cronInstance.Schedule(schedule, cron.FuncJob(func() {
		ExecuteJob(&j)
	}))
	entryIDs[job.ID] = id
}

// NextRuns 计算表达式未来 n 次触发时间（用于预览与列表富化）。
func NextRuns(expr string, n int) ([]time.Time, error) {
	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	runs := make([]time.Time, 0, n)
	t := now
	for i := 0; i < n; i++ {
		t = schedule.Next(t)
		runs = append(runs, t)
	}
	return runs, nil
}

// ExecuteJob 立即执行一次任务并记录运行日志。
func ExecuteJob(job *CronJob) {
	started := time.Now()
	output, errMsg, success := runAction(job)
	finished := time.Now()

	run := CronJobRun{
		JobID:      job.ID,
		StartedAt:  started,
		FinishedAt: &finished,
		Success:    success,
		Output:     truncate(output, 8000),
		Error:      truncate(errMsg, 2000),
	}
	if err := config.GetDB().Create(&run).Error; err != nil {
		slog.Error("写入定时任务运行日志失败", "jobId", job.ID, "err", err)
	}

	status := "ok"
	if !success {
		status = "failed"
	}
	if err := config.GetDB().Model(&CronJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"last_run_at": &finished,
		"last_status": status,
		"updated_at":  finished,
	}).Error; err != nil {
		slog.Error("更新定时任务运行状态失败", "jobId", job.ID, "err", err)
	}
}

func runAction(job *CronJob) (output, errMsg string, success bool) {
	switch job.ActionType {
	case ActionTypeShell:
		if !config.GetConfig().FeatureCronShell {
			return "", "命令执行功能未启用（FEATURE_CRON_SHELL=false）", false
		}
		return runShell(job)
	case ActionTypeHTTP:
		return runHTTP(job)
	default:
		return "", "未知动作类型: " + string(job.ActionType), false
	}
}

func runHTTP(job *CronJob) (output, errMsg string, success bool) {
	var act HTTPAction
	if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
		return "", "解析 HTTP 动作配置失败: " + err.Error(), false
	}
	if act.Method == "" {
		act.Method = http.MethodGet
	}
	url := act.URL
	if len(url) > 0 && url[0] == '/' {
		url = selfBaseURL() + url
	}
	req, err := http.NewRequest(act.Method, url, strings.NewReader(act.Body))
	if err != nil {
		return "", "构造请求失败: " + err.Error(), false
	}
	for k, v := range act.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: time.Duration(job.TimeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "请求失败: " + err.Error(), false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var sb strings.Builder
	fmt.Fprintf(&sb, "HTTP %d\n", resp.StatusCode)
	sb.Write(body)
	if resp.StatusCode >= 400 {
		return sb.String(), fmt.Sprintf("HTTP %d", resp.StatusCode), false
	}
	return sb.String(), "", true
}

func selfBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", config.GetConfig().Server.Port)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... (截断，总长 %d 字符)", len(s))
}
