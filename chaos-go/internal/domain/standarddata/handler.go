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
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.RegisterV1("standard-data", RegisterV1)
}

// Register 把本资源的 RESTful 路由挂载到给定路由组（存量 /api）。
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

// RegisterV1 把本资源以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 复用通用 crud 的 RegisterActions，动作集：list/get/create/update/delete/
// batchCreate/batchDelete/status；与 Register 的存量路由双轨并存。
func RegisterV1(rg *gin.RouterGroup) {
	crud.RegisterActions[StandardData](rg, "standard-data", crud.Opts[StandardData]{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
	}, &crud.ToggleOpts{
		Setter: ToggleStatus,
		ErrRules: []httpx.ErrRule{
			{Err: ErrRecordNotFound, Status: http.StatusNotFound, Msg: "记录不存在"},
			{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
		},
	})
}
