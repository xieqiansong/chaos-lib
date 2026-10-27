package project

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "project",
			Name:         "项目管理",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&ProjectGroup{}, &Project{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
