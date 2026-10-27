package stunpf

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
)

// init 把 STUN 端口转发同步登记为平台插件：它没有路由与模型，只提供自己的周期任务。
// 任务由本模块拥有（而不是写进 cron_jobs 表），未启用或数据库不可用时 RunSync 内部直接跳过。
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
