package dict

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

var errRules = []httpx.ErrRule{
	{Err: ErrDictTypeNotFound, Status: http.StatusNotFound, Msg: "字典类型不存在"},
	{Err: ErrDictDataNotFound, Status: http.StatusNotFound, Msg: "字典数据不存在"},
	{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
}

func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[DictType](rg, "dict-types", crud.Opts[DictType]{
		Searchable: []string{"name", "code", "remark"},
		Sortable:   []string{"id", "sort", "created_at"},
	}, &crud.ToggleOpts{
		Setter:   SetDictTypeStatus,
		ErrRules: errRules,
	})

	g := crud.RegisterActions[DictData](rg, "dict-data", crud.Opts[DictData]{
		Searchable: []string{"label", "value", "remark"},
		Sortable:   []string{"id", "sort", "created_at"},
	}, &crud.ToggleOpts{
		Setter:   SetDictDataStatus,
		ErrRules: errRules,
	})

	g.POST("/tree", treeAction)
	g.POST("/options", optionsAction)
}

func treeAction(c *web.Context) {
	var req struct {
		TypeCode string `json:"typeCode"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.TypeCode == "" {
		renv.Error(c, http.StatusBadRequest, "缺少 typeCode")
		return
	}
	nodes, err := BuildTree(req.TypeCode)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, nodes)
}

func optionsAction(c *web.Context) {
	var req struct {
		TypeCode string `json:"typeCode"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.TypeCode == "" {
		renv.Error(c, http.StatusBadRequest, "缺少 typeCode")
		return
	}
	opts, err := ListOptions(req.TypeCode)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, opts)
}
