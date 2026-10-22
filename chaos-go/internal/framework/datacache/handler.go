package datacache

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件：模型与路由挂载交由插件框架编排（plugin.ModelsOf / plugin.MountAll），
// 不再经 routehub 二次登记，避免重复挂载；DataCache 模型也在此声明，统一由装配层迁移。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "datacache",
			Name:         "数据缓存",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&DataCache{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}

// dataCacheOpts 标准 CRUD 选项，存量 /api 与 v1 动作路由共用。
var dataCacheOpts = crud.Opts[DataCache]{
	// Value 为二进制列，无法做文本 LIKE 搜索，故仅 category / key / data_type 可搜。
	Searchable: []string{"category", "key", "data_type"},
	Sortable:   []string{"id", "value_len", "created_at"},
}

// RegisterV1 把本资源以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[DataCache](rg, "data-caches", dataCacheOpts, nil)
}
