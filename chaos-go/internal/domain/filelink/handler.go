package filelink

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"chaos-go/internal/framework/web"
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
	BeforeCreate: ValidateForCreate,
}
var fileLinkToggle = &crud.ToggleOpts{
	Setter:   ToggleStatus,
	ErrRules: linkErrRules,
}

// RegisterV1 把本资源以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// CRUD + status（启停）由通用 crud 生成，动作名：list/get/create/update/delete/batchCreate/batchDelete/status。
// statusOf 为读方向自定义动作：列表加载后前端逐行并发调用，按文件系统实时计算单条 LinkStatus。
func RegisterV1(rg *web.RouterGroup) {
	fl := crud.RegisterActions[FileLink](rg, "file-links", fileLinkOpts, fileLinkToggle)
	fl.POST("/statusOf", statusOfV1)
}

// statusOfV1 逐行状态刷新接口（POST /api/v1/file-links/statusOf，body {id}）。
// 对应前端「列表加载完成后单独刷新每一行状态」的需求；返回 { LinkStatus }。
func statusOfV1(c *web.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		resp.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	status, err := GetLinkStatus(req.ID)
	if err != nil {
		httpx.MapError(c, err, linkErrRules, http.StatusBadRequest)
		return
	}
	resp.Success(c, map[string]any{"LinkStatus": status})
}
