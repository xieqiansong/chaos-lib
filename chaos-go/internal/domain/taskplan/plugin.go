package taskplan

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "taskplan",
			Name:         "任务计划",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&TaskPlan{}, &Task{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
		Tasks: []scheduler.TaskSpec{
			{
				Name:       "任务计划扫描",
				Cron:       "0 */5 * * * *",
				Timeout:    30 * time.Second,
				SkipIfBusy: true,
				Run: func(ctx context.Context) error {
					SweepScheduledTaskPlans()
					return nil
				},
			},
		},
	})
}
