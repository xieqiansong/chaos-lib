package proxy

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件（与 portfwd / taskplan 等对齐）。
// 原先 handler_v1.go 中的 routehub.RegisterV1 调用已移除，路由改由插件框架的
// RegisterRoutes 统一挂载（plugin.MountAll 内部调用 RegisterV1(rg)）。
// 模型（BrowserHistory / BrowserHistoryVisit / Bookmark）一并声明，由插件框架统一 AutoMigrate。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "proxy",
			Name:         "代理服务",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&BrowserHistory{}, &BrowserHistoryVisit{}, &Bookmark{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
