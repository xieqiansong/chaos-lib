package filelink

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "filelink",
			Name:         "文件链接",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&FileLink{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
