package crud

import (
	"net/http"

	"chaos-go/internal/httpx"
	renv "chaos-go/internal/resp"

	"github.com/gin-gonic/gin"
)

// ToggleOpts 启停子路由（PATCH /:id/status）的配置。
type ToggleOpts struct {
	// Setter 业务实现：按 ID 与目标状态执行切换（含副作用），返回响应载荷。
	// 编排属于用例层，业务包应把它实现在 service.go 里，这里只做一行转调。
	Setter func(id int, status bool) (any, error)
	// ErrRules 领域错误 → HTTP 状态码映射表；未命中回落 500。
	ErrRules []httpx.ErrRule
}

// RegisterToggle 在给定资源路由组下挂载 PATCH /:id/status 启停子路由。
//
// standarddata / cronjob / filelink / portfwd 四个包都要写一遍
// 「绑 {status:bool} → 解析 ID → 执行切换 → 映射错误 → 响应」，骨架逐字相同，
// 差异只在切换动作与错误表，故收进基线：业务包只提供 Setter 与 ErrRules。
func RegisterToggle(g *gin.RouterGroup, opts ToggleOpts) {
	g.PATCH("/:id/status", func(c *gin.Context) {
		var req struct {
			Status bool `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		id, ok := httpx.ParseID(c)
		if !ok {
			return
		}
		row, err := opts.Setter(id, req.Status)
		if err != nil {
			httpx.MapError(c, err, opts.ErrRules)
			return
		}
		renv.Success(c, row)
	})
}
