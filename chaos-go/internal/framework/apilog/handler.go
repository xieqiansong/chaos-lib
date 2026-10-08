package apilog

import (
	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/framework/web"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.RegisterV1("api-logs", RegisterV1)
}

// apiLogOpts 标准 CRUD 选项，存量 /api 与 v1 动作路由共用。
var apiLogOpts = crud.Opts[ApiLog]{
	Searchable: []string{"path", "method", "error_msg"},
	Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
}

// RegisterV1 把本资源以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[ApiLog](rg, "api-logs", apiLogOpts, nil)
}
