package filelink

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
	routehub.RegisterV1("file-links", RegisterV1)
}

// linkErrRules 启停相关领域错误 → HTTP 状态码映射表（未命中回落 500）。
var linkErrRules = []httpx.ErrRule{
	{Err: ErrLinkNotFound, Status: http.StatusNotFound, Msg: "文件连接不存在"},
	{Err: ErrAlreadyEnabled, Status: http.StatusBadRequest},
	{Err: ErrAlreadyDisabled, Status: http.StatusBadRequest},
	{Err: ErrSourceMissing, Status: http.StatusBadRequest},
	{Err: ErrTargetExists, Status: http.StatusBadRequest},
}

// fileLinkOpts / fileLinkToggle 标准 CRUD 与启停选项，存量 /api 与 v1 动作路由共用。
var fileLinkOpts = crud.Opts[FileLink]{
	Searchable: []string{"source_path", "target_path", "remark"},
	Sortable:   []string{"id", "sort"},
	// Status 只能走 /status（带建删联接点副作用），禁止通用 PATCH 改写
	Protected:    []string{"status"},
	ToResponse:   toResponse,
	BeforeCreate: ValidateForCreate,
	AfterDelete:  CleanupLink,
}
var fileLinkToggle = &crud.ToggleOpts{
	Setter:   ToggleStatus,
	ErrRules: linkErrRules,
}

// RegisterV1 把本资源以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// CRUD + status（启停）由通用 crud 生成，动作名：list/get/create/update/delete/batchCreate/batchDelete/status。
func RegisterV1(rg *gin.RouterGroup) {
	crud.RegisterActions[FileLink](rg, "file-links", fileLinkOpts, fileLinkToggle)
}
