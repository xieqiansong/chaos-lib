package standarddata

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("standard-data", Register)
}

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 纯 CRUD 交给通用 crud；状态切换走基线的通用启停路由，本包只提供切换动作与错误表。
func Register(rg *gin.RouterGroup) {
	g := crud.Register[StandardData](rg, "standard-data", crud.Opts[StandardData]{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
	})
	crud.RegisterToggle(g, crud.ToggleOpts{
		Setter: ToggleStatus,
		ErrRules: []httpx.ErrRule{
			{Err: ErrRecordNotFound, Status: http.StatusNotFound, Msg: "记录不存在"},
			{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
		},
	})
}
