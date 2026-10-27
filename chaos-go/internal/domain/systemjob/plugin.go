package systemjob

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "systemjob",
			Name:         "系统任务",
			Version:      "1.0.0",
			Dependencies: []string{"taskplan", "portfwd"},
		},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
