package cronjob

import "time"

// CronJobView 是本资源的响应形态：模型 + 派生的下次执行时间。
// NextRun 由 service 按 cron 表达式实时计算后注入（不落库）。
type CronJobView struct {
	CronJob
	NextRun *time.Time `json:"NextRun"`
}

// toView 组装响应视图：模型 + 可选的下次执行时间。
func toView(job CronJob, nextRun *time.Time) CronJobView {
	return CronJobView{CronJob: job, NextRun: nextRun}
}
