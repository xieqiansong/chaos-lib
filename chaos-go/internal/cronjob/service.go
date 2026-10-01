package cronjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"chaos-go/internal/config"

	"github.com/robfig/cron/v3"
)

// ── 内部工具 ──

func jsonUnmarshal(s string, v interface{}) error {
	if s == "" {
		return fmt.Errorf("配置为空")
	}
	return json.Unmarshal([]byte(s), v)
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func selfBaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", config.GetConfig().Server.Port)
}

// defaultTimeoutSec 是未显式配置超时时的兜底值（秒）。
const defaultTimeoutSec = 30

// timeoutOf 兜底超时：未配置（0）或非法值时按 30 秒计，避免 http.Client 零超时等于不超时。
func timeoutOf(job *CronJob) time.Duration {
	if job.TimeoutSec <= 0 {
		return defaultTimeoutSec * time.Second
	}
	return time.Duration(job.TimeoutSec) * time.Second
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... (截断，总长 %d 字符)", len(s))
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

// afterSave 在 create / update 事务内、提交前执行：先校验（失败即回滚，
// 既不写库也不进调度器），通过后再把最新状态同步进 cron 调度器。
func afterSave(row *CronJob) error {
	if err := validateJob(row); err != nil {
		return err
	}
	SyncJob(row)
	return nil
}

// ── 调度器 ──

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
	jobs, err := FindEnabledJobs()
	if err != nil {
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

// ── 执行 ──

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
	if err := CreateRun(&run); err != nil {
		slog.Error("写入定时任务运行日志失败", "jobId", job.ID, "err", err)
	}

	status := "ok"
	if !success {
		status = "failed"
	}
	if err := UpdateJobRunResult(job.ID, finished, status); err != nil {
		slog.Error("更新定时任务运行状态失败", "jobId", job.ID, "err", err)
	}
}

func runAction(job *CronJob) (output, errMsg string, success bool) {
	switch job.ActionType {
	case ActionTypeShell:
		if !config.GetConfig().Feature.CronShell {
			return "", "命令执行功能未启用（feature.cron_shell=false）", false
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
	client := &http.Client{Timeout: timeoutOf(job)}
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

// runShell 执行命令（预留功能，仅当 FEATURE_CRON_SHELL=true 时由 runAction 调用）。
func runShell(job *CronJob) (output, errMsg string, success bool) {
	var act ShellAction
	if err := jsonUnmarshal(job.ActionConfig, &act); err != nil {
		return "", "解析 Shell 动作配置失败: " + err.Error(), false
	}
	if strings.TrimSpace(act.Command) == "" {
		return "", "命令为空", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutOf(job))
	defer cancel()

	args := append([]string{"/c", act.Command}, act.Args...)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", args...)
	} else {
		cmd = exec.CommandContext(ctx, "sh", append([]string{"-c", act.Command}, act.Args...)...)
	}
	if act.WorkDir != "" {
		cmd.Dir = act.WorkDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), "执行失败: " + err.Error(), false
	}
	return string(out), "", true
}

// ── 默认数据 ──

// SeedDefaults 在库内无任何定时任务时，写入由原系统内置周期任务转换而来的默认任务，
// 保证「改为 API + 由本模块触发」后原有行为不中断。
func SeedDefaults() {
	count, err := CountJobs()
	if err != nil {
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
	if err := CreateJobs(defaults); err != nil {
		slog.Error("写入默认定时任务失败", "err", err)
		return
	}
	slog.Info("已写入默认定时任务", "count", len(defaults))
}
