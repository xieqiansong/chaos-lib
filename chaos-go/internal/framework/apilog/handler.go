package apilog

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/framework/web"
)

func init() {
	routehub.RegisterV1("api-logs", RegisterV1)
}

var apiLogOpts = crud.Opts[ApiLog]{
	Searchable: []string{"path", "method", "error_msg"},
	Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
}

func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[ApiLog](rg, "api-logs", apiLogOpts, nil)
}
