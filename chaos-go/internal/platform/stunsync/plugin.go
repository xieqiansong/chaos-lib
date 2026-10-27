package stunsync

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
)

// init 把 STUN 规则同步登记为平台插件：它没有路由与模型，只提供自己的周期任务。
// 任务由本模块拥有（而不是写进 cron_jobs 表），未启用时 RunSync 内部直接跳过。
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
