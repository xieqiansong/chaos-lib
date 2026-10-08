package cronjob

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"chaos-go/internal/framework/config"
)

// 本文件是任务动作的执行器：按动作类型执行并落运行日志。只被 service 调用。

// defaultTimeoutSec 是未显式配置超时时的兜底值（秒）。
const defaultTimeoutSec = 30

func jsonUnmarshal(s string, v interface{}) error {
	if s == "" {
		return fmt.Errorf("配置为空")
	}
	return json.Unmarshal([]byte(s), v)
}

func selfBaseURL() string {
	if config.GetConfig().Server.EnableHTTP3 {
		return fmt.Sprintf("https://localhost:%d", config.GetConfig().Server.Port)
	} else {
		return fmt.Sprintf("http://localhost:%d", config.GetConfig().Server.Port)
	}
}

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

// ExecuteJob 立即执行一次任务并记录运行日志，返回本次运行记录。
func ExecuteJob(job *CronJob) (*CronJobRun, error) {
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
		return nil, fmt.Errorf("写入运行日志失败: %w", err)
	}

	status := "ok"
	if !success {
		status = "failed"
	}
	if err := UpdateJobRunResult(job.ID, finished, status); err != nil {
		// 运行日志已落库，状态回写失败只记日志，不影响本次结果返回。
		slog.Error("更新定时任务运行状态失败", "jobId", job.ID, "err", err)
	}
	return &run, nil
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
