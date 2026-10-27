package stunpf

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "stunpf",
			Name:         "STUN 端口转发同步",
			Version:      "1.0.0",
			Dependencies: []string{"mqttsync", "portfwd"},
		},
		Tasks: []scheduler.TaskSpec{
			{
				Name:       "STUN 端口转发同步",
				Cron:       "*/15 * * * * *",
				Timeout:    30 * time.Second,
				SkipIfBusy: true,
				Run: func(ctx context.Context) error {
					RunSync()
					return nil
				},
			},
		},
	})
}
