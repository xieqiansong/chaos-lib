package project

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除，
// 路由改由插件框架的 RegisterRoutes 统一挂载（plugin.MountAll 内部调用 RegisterV1(rg)），
// 不再经 routehub 二次登记，避免双重挂载与漏挂。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "project",
			Name:         "项目管理",
			Version:      "1.0.0",
			Dependencies: nil, // 仅依赖 framework 隐式根，无业务插件依赖
		},
		Models: []any{&ProjectGroup{}, &Project{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
		// 本轮 project 无后台任务；Configure/Start/Stop 留 nil。
	})
}
