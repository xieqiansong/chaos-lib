package proxy

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "proxy",
			Name:         "代理服务",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&BrowserHistory{}, &BrowserHistoryVisit{}, &Bookmark{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
