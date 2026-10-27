package cronjob

import "time"

type CronJobView struct {
	CronJob
	NextRun *time.Time `json:"NextRun"`
}

func toView(job CronJob, nextRun *time.Time) CronJobView {
	return CronJobView{CronJob: job, NextRun: nextRun}
}
