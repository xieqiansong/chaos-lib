package dict

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把数据字典模块登记为插件：声明两张表模型并挂载路由。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "dict",
			Name:         "数据字典",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&DictType{}, &DictData{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
