package apilog

import (
	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
)

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 纯 CRUD 交给通用 crud，前端按路径/方法/错误信息搜索、按耗时/状态排序。
func Register(rg *gin.RouterGroup) {
	crud.Register[ApiLog](rg, "apiLog", crud.Opts[ApiLog]{
		Searchable: []string{"path", "method", "error_msg"},
		Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
	})
}
