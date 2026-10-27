package filelink

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

var linkErrRules = []httpx.ErrRule{
	{Err: ErrLinkNotFound, Status: http.StatusNotFound, Msg: "文件连接不存在"},
	{Err: ErrAlreadyEnabled, Status: http.StatusBadRequest},
	{Err: ErrAlreadyDisabled, Status: http.StatusBadRequest},
	{Err: ErrSourceMissing, Status: http.StatusBadRequest},
	{Err: ErrTargetExists, Status: http.StatusBadRequest},
}

var fileLinkOpts = crud.Opts[FileLink]{
	Searchable: []string{"source_path", "target_path", "remark"},
	Sortable:   []string{"id", "sort"},

	Protected:    []string{"status"},
	BeforeCreate: ValidateForCreate,
}
var fileLinkToggle = &crud.ToggleOpts{
	Setter:   ToggleStatus,
	ErrRules: linkErrRules,
}

func RegisterV1(rg *web.RouterGroup) {
	fl := crud.RegisterActions[FileLink](rg, "file-links", fileLinkOpts, fileLinkToggle)
	fl.POST("/statusOf", statusOfV1)
}

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
