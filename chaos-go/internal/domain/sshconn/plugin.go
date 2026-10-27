package sshconn

import "chaos-go/internal/framework/plugin"

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "sshconn",
			Name:         "SSH连接",
			Version:      "1.0.0",
			Dependencies: nil,
		},
	})
}
