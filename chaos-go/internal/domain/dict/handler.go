package dict

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

// errRules 领域错误 → HTTP 状态码映射。
var errRules = []httpx.ErrRule{
	{Err: ErrDictTypeNotFound, Status: http.StatusNotFound, Msg: "字典类型不存在"},
	{Err: ErrDictDataNotFound, Status: http.StatusNotFound, Msg: "字典数据不存在"},
	{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
}

// RegisterV1 挂载数据字典模块到 /api/v1（POST + Action 风格）。
// 两张表走通用 CRUD（含 status 启停）；dict-data 另挂 tree / options 两个只读派生动作。
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

	// 派生查询（只读）
	g.POST("/tree", treeAction)
	g.POST("/options", optionsAction)
}

// treeAction 返回某类型下的字典数据项树（按 ParentID 嵌套）。
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

// optionsAction 返回某类型下启用状态的字典数据项（供其他模块下拉）。
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
