package standarddata

import (
	"net/http"

	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("standardData", Register)
}

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 纯 CRUD 交给通用 crud；状态切换在基线之外由本包自实现并挂载，避免污染标准实现。
func Register(rg *gin.RouterGroup) {
	crud.Register[StandardData](rg, "standardData", crud.Opts[StandardData]{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
	})
	// 自定义子路由：状态切换（标准 CRUD 之外的本业务实现）
	rg.Group("/standardData").PATCH("/:id/status", status)
}

// status 状态切换（本业务包的自定义子路由实现：PATCH /standardData/:id/status，body {status:bool}）。
func status(c *gin.Context) {
	var req struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	id := c.Param("id")
	if _, err := FindActiveByID(id); err != nil {
		renv.Error(c, http.StatusNotFound, "记录不存在")
		return
	}
	row, err := SetStatus(id, req.Status)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "状态更新失败: "+err.Error())
		return
	}
	renv.Success(c, row)
}
