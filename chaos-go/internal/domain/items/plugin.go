package items

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把个人物品模块登记为插件：声明 5 张表模型并挂载路由。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "items",
			Name:         "个人物品",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&Category{}, &Location{}, &Item{}, &ItemRecord{}, &ItemAssembly{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
