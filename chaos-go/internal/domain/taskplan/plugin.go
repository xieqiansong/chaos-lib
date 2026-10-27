package taskplan

import (
	"context"
	"time"

	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件：声明元数据与依赖，并把路由挂载函数交给插件框架编排。
// RegisterRoutes 直接在本分组上建子路由（RegisterV1(rg)），不再经 routehub 二次登记；
// 原先 handler_v1.go 中独立的 routehub.RegisterV1 调用已移除，避免重复登记与漏挂。
// 「任务计划扫描」是模块自带的周期任务（Tasks），由 framework/scheduler 运行，不落 cron_jobs 表。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "taskplan",
			Name:         "任务计划",
			Version:      "1.0.0",
			Dependencies: nil, // 仅依赖 framework 隐式根，无业务插件依赖
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
