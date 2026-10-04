package datacache

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("data-caches", Register)
}

// Register 把本资源的标准 CRUD 路由挂载到给定路由组。
// 仅纯 CRUD，无扩展子路由，全部交给通用 crud 基线。
func Register(rg *gin.RouterGroup) {
	crud.Register[DataCache](rg, "data-caches", crud.Opts[DataCache]{
		// Value 为二进制列，无法做文本 LIKE 搜索，故仅 category / key / data_type 可搜。
		Searchable: []string{"category", "key", "data_type"},
		Sortable:   []string{"id", "value_len", "created_at"},
	})
}
