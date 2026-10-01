package cronjob

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// 默认数据：库内无任务时写入由原内置周期任务转换而来的默认任务，
// 保证「改为 API + 由本模块触发」后原有行为不中断。

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// SeedDefaults 在库内无任何定时任务时写入默认任务。
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
