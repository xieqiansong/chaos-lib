package dashboard

import "chaos-go/internal/framework/plugin"

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "dashboard",
			Name:         "看板",
			Version:      "1.0.0",
			Dependencies: nil,
		},
	})
}
