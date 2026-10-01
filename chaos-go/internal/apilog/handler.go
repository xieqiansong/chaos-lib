package apilog

import (
	"chaos-go/internal/crud"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("apiLog", Register)
}

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 纯 CRUD 交给通用 crud，前端按路径/方法/错误信息搜索、按耗时/状态排序。
func Register(rg *gin.RouterGroup) {
	crud.Register[ApiLog](rg, "apiLog", crud.Opts[ApiLog]{
		Searchable: []string{"path", "method", "error_msg"},
		Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
	})
}
