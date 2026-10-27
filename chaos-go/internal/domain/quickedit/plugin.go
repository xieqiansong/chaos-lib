package quickedit

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

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
