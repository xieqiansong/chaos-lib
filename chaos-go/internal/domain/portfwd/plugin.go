package portfwd

import (
	"context"
	"time"

	"chaos-go/internal/framework/database"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除，
// 路由改由插件框架的 RegisterRoutes 统一挂载（plugin.MountAll 内部调用 RegisterV1(rg)）。
// 进程重启后的隧道恢复（RecoverForwardsOnBoot）从 app.go 显式调用下沉到 Start 钩子，
// 由插件框架按拓扑序在启动时统一编排。
// 周期自愈任务由本插件自带（Tasks），不再写入 cron_jobs 表。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "portfwd",
			Name:         "端口转发",
			Version:      "1.0.0",
			Dependencies: []string{"database"},
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
					// 数据库不可用时直接跳过本轮，避免把「依赖未就绪」记成任务失败。
					if !database.Ready() {
						return nil
					}
					SelfHealForwards()
					return nil
				},
			},
		},
		Start: func() error {
			// 数据库未就绪时先登记回调：连接成功后再补做隧道恢复，
			// 避免启动期数据库故障导致存量转发规则永久不恢复。
			if database.Ready() {
				RecoverForwardsOnBoot()
			} else {
				database.OnReady(RecoverForwardsOnBoot)
			}
			return nil
		},
	})
}
