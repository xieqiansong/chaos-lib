package standarddata

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/web"
)

func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[StandardData](rg, "standard-data", crud.Opts[StandardData]{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
	}, &crud.ToggleOpts{
		Setter: ToggleStatus,
		ErrRules: []httpx.ErrRule{
			{Err: ErrRecordNotFound, Status: http.StatusNotFound, Msg: "记录不存在"},
			{Err: ErrDBUnavailable, Status: http.StatusInternalServerError, Msg: "数据库不可用"},
		},
	})
}
