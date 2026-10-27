package envvar

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

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
