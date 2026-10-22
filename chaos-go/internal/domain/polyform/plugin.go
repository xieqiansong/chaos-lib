package polyform

import "chaos-go/internal/framework/plugin"

// init 把本模块登记为插件。polyform 为前端格式转换视图，无独立后端模型与路由，
// 仅作插件占位以对齐前后端插件 ID。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "polyform",
			Name:         "格式转换",
			Version:      "1.0.0",
			Dependencies: nil,
		},
	})
}
