package quickedit

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除。
// quickedit 运行期依赖 envvar 提供的虚拟文件服务（经 app.go 注入 EnvReadContent /
// EnvWriteContent 回调），此处以 SoftDeps 记录该软依赖（不参与硬环判定）。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "quickedit",
			Name:         "快速编辑",
			Version:      "1.0.0",
			Dependencies: nil,
			SoftDeps:     []string{"envvar"},
		},
		Models: []any{&QuickEditFile{}, &QuickEditSnapshot{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
