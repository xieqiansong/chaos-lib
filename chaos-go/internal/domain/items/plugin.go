package items

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "items",
			Name:         "个人物品",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&Item{}, &ItemRecord{}, &ItemAssembly{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
