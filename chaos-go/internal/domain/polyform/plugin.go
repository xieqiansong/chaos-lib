package polyform

import "chaos-go/internal/framework/plugin"

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "polyform",
			Name:         "格式转换",
			Version:      "1.0.0",
			Dependencies: nil,
		},
	})
}
