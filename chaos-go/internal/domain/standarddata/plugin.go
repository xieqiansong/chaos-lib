package standarddata

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

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
