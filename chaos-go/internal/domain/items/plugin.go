package items

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把个人物品模块登记为插件：声明 3 张表模型并挂载路由。
// 物品分类 / 存放位置 / 购入来源已迁至数据字典（dict），故不再在此托管其表。
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
