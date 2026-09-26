package cronjob

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

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
