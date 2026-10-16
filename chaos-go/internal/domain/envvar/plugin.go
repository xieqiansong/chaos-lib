package envvar

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除，
// 路由改由插件框架的 RegisterRoutes 统一挂载（plugin.MountAll 内部调用 RegisterV1(rg)）。
// 注意：envvar 以 TOML / 虚拟文件（经 quickedit API）存储，无 GORM 模型，Models 留空。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "envvar",
			Name:         "环境变量",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
