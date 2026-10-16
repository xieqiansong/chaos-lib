package systemjob

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 systemjob.go 中的 routehub.RegisterV1 调用已移除。
// 本包不持有模型，仅做跨模块动作编排（依赖已插件化的 taskplan / portfwd；
// stunpf / stunsync 仍为非插件平台服务，故不在 Dependencies 中声明）。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "systemjob",
			Name:         "系统任务",
			Version:      "1.0.0",
			Dependencies: []string{"taskplan", "portfwd"},
		},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
