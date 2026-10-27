package portfwd

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
			ID:           "portfwd",
			Name:         "端口转发",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&PortForwarding{}, &SshConnection{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
		Tasks: []scheduler.TaskSpec{
			{
				Name:       "端口转发自愈",
				Cron:       "0/15 * * * * *",
				Timeout:    60 * time.Second,
				SkipIfBusy: true,
				Run: func(ctx context.Context) error {
					SelfHealForwards()
					return nil
				},
			},
		},
		Start: func() error {
			RecoverForwardsOnBoot()
			return nil
		},
	})
}
