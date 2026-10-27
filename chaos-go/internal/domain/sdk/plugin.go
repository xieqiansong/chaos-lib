package sdk

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "sdk",
			Name:         "SDK版本",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&SdkSource{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
