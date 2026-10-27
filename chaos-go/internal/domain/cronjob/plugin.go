package cronjob

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "cronjob",
			Name:         "定时任务",
			Version:      "1.0.0",
			Dependencies: []string{"systemjob"},
		},
		Models: []any{&CronJob{}, &CronJobRun{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
		Start: func() error {
			Start()
			return nil
		},
	})
}
