package cronjob

import (
	"chaos-go/internal/framework/database"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除。
// Start 钩子接管原 app.go 的 cronjob.Start()（cron 调度运行时启动），改由插件框架按拓扑序编排。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "cronjob",
			Name:         "定时任务",
			Version:      "1.0.0",
			Dependencies: []string{"database"},
		},
		Models: []any{&CronJob{}, &CronJobRun{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
		Start: func() error {
			// Start 幂等：数据库已就绪则立即加载用户任务；
			// 未就绪则登记回调，由 database 插件在连接成功后补加载。
			if database.Ready() {
				Start()
			} else {
				database.OnReady(Start)
			}
			return nil
		},
	})
}
