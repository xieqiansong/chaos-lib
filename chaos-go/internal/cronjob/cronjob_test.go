package cronjob

import (
	"strings"
	"testing"
)

// newTestJob 构造一个合法的 http 类型任务（供各用例按需破坏某个字段）。
func newTestJob() *CronJob {
	return &CronJob{
		Name:         "测试任务",
		CronExpr:     "*/5 * * * *",
		ActionType:   ActionTypeHTTP,
		ActionConfig: mustJSON(HTTPAction{Method: "POST", URL: "/api/systemJobs/sweep"}),
		Enabled:      true,
		TimeoutSec:   30,
	}
}

func TestValidateJob(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(j *CronJob)
		wantErr string
	}{
		{"合法任务", func(j *CronJob) {}, ""},
		{"名称为空", func(j *CronJob) { j.Name = "  " }, "任务名称不能为空"},
		{"表达式为空", func(j *CronJob) { j.CronExpr = "" }, "cron 表达式不能为空"},
		{"表达式非法", func(j *CronJob) { j.CronExpr = "*/5 * *" }, "无效的 cron 表达式"},
		{"超时为负", func(j *CronJob) { j.TimeoutSec = -1 }, "超时时间不能为负数"},
		{"未知动作类型", func(j *CronJob) { j.ActionType = "email" }, "未知动作类型"},
		{"http 缺 URL", func(j *CronJob) {
			j.ActionConfig = mustJSON(HTTPAction{Method: "POST"})
		}, "HTTP 动作的 URL 不能为空"},
		{"动作配置非 JSON", func(j *CronJob) { j.ActionConfig = "not-json" }, "解析 HTTP 动作配置失败"},
		{"命令为空", func(j *CronJob) {
			j.ActionType = ActionTypeShell
			j.ActionConfig = mustJSON(ShellAction{})
		}, "命令动作的命令不能为空"},
		{"合法命令任务", func(j *CronJob) {
			j.ActionType = ActionTypeShell
			j.ActionConfig = mustJSON(ShellAction{Command: "echo hi"})
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job := newTestJob()
			c.mutate(job)
			err := validateJob(job)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("期望校验通过，实际: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("期望错误包含 %q，实际: %v", c.wantErr, err)
			}
		})
	}
}

func TestToResponseNextRun(t *testing.T) {
	enabled := newTestJob()
	disabled := newTestJob()
	disabled.Enabled = false

	out, ok := toResponse([]*CronJob{enabled, disabled}).([]CronJobView)
	if !ok {
		t.Fatalf("toResponse 应返回 []CronJobView")
	}
	if len(out) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(out))
	}
	if out[0].NextRun == nil {
		t.Fatal("已启用任务应带下次执行时间")
	}
	if !out[0].NextRun.After(out[0].CreatedAt) {
		t.Fatalf("下次执行时间应晚于创建时间: %v", out[0].NextRun)
	}
	if out[1].NextRun != nil {
		t.Fatalf("未启用任务不应计算下次执行时间，实际 %v", out[1].NextRun)
	}
}

func TestNextRuns(t *testing.T) {
	// 5 字段（分 时 日 月 周）
	runs, err := NextRuns("*/5 * * * *", 3)
	if err != nil {
		t.Fatalf("5 字段表达式解析失败: %v", err)
	}
	if len(runs) != 3 || runs[0].Minute()%5 != 0 {
		t.Fatalf("5 字段表达式的未来时间不符合预期: %v", runs)
	}
	// 6 字段含秒：相邻两次相差 30 秒
	runs, err = NextRuns("*/30 * * * * *", 2)
	if err != nil {
		t.Fatalf("6 字段表达式解析失败: %v", err)
	}
	if d := runs[1].Sub(runs[0]); d.Seconds() != 30 {
		t.Fatalf("期望间隔 30 秒，实际 %v", d)
	}
	if _, err := NextRuns("bad expr", 1); err == nil {
		t.Fatal("非法表达式应返回错误")
	}
}

func TestTimeoutOf(t *testing.T) {
	if got := timeoutOf(&CronJob{TimeoutSec: 0}); got.Seconds() != defaultTimeoutSec {
		t.Fatalf("未配置超时应兜底为 %d 秒，实际 %v", defaultTimeoutSec, got)
	}
	if got := timeoutOf(&CronJob{TimeoutSec: 5}); got.Seconds() != 5 {
		t.Fatalf("显式超时未生效，实际 %v", got)
	}
}
