package standarddata

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除，
// 路由改由插件框架的 RegisterRoutes 统一挂载。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "standarddata",
			Name:         "标准数据",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&StandardData{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
