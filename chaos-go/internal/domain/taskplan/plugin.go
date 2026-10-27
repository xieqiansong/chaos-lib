package taskplan

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件：声明元数据与依赖，并把路由挂载函数交给插件框架编排。
// RegisterRoutes 直接在本分组上建子路由（RegisterV1(rg)），不再经 routehub 二次登记；
// 原先 handler_v1.go 中独立的 routehub.RegisterV1 调用已移除，避免重复登记与漏挂。
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
		// 本轮 taskplan 无后台任务；Configure/Start/Stop 留 nil。
		// 后续 systemjob / cronjob 等会在各自插件中演示 Start 接入 scheduler。
	})
}
