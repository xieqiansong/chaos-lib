package stunsync

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "stunsync",
			Name:         "STUN 规则同步",
			Version:      "1.0.0",
			Dependencies: []string{"mqttsync"},
		},
		Tasks: []scheduler.TaskSpec{
			{
				Name:       "STUN 规则同步",
				Cron:       "0 * * * * *",
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
