package dashboard

import "chaos-go/internal/framework/plugin"

// init 把本模块登记为插件。dashboard 为前端聚合视图（依赖 proxy / task-plans 等既有模块接口），
// 无独立后端模型与路由，仅作插件占位以对齐前后端插件 ID。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "dashboard",
			Name:         "看板",
			Version:      "1.0.0",
			Dependencies: nil,
		},
	})
}
