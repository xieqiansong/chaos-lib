package filelink

import (
	"net/http"

	"chaos-go/internal/crud"
	"chaos-go/internal/httpx"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("file-links", Register)
}

// linkErrRules 启停相关领域错误 → HTTP 状态码映射表（未命中回落 500）。
var linkErrRules = []httpx.ErrRule{
	{Err: ErrLinkNotFound, Status: http.StatusNotFound, Msg: "文件连接不存在"},
	{Err: ErrAlreadyEnabled, Status: http.StatusBadRequest},
	{Err: ErrAlreadyDisabled, Status: http.StatusBadRequest},
	{Err: ErrSourceMissing, Status: http.StatusBadRequest},
	{Err: ErrTargetExists, Status: http.StatusBadRequest},
}

// Register 把本资源的路由挂载到给定路由组。
// 纯 CRUD 交给通用 crud；状态切换走基线的通用启停路由，本包只提供切换动作与错误表。
func Register(rg *gin.RouterGroup) {
	g := crud.Register[FileLink](rg, "file-links", crud.Opts[FileLink]{
		Searchable: []string{"source_path", "target_path", "remark"},
		Sortable:   []string{"id", "sort"},
		// Status 只能走 /:id/status（带建删联接点副作用），禁止通用 PATCH 改写
		Protected:    []string{"status"},
		ToResponse:   toResponse,
		BeforeCreate: ValidateForCreate,
		AfterDelete:  CleanupLink,
	})
	crud.RegisterToggle(g, crud.ToggleOpts{
		Setter:   ToggleStatus,
		ErrRules: linkErrRules,
	})
}
