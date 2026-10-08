package datacache

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/framework/web"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.RegisterV1("data-caches", RegisterV1)
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
