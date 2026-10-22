package sshconn

import "chaos-go/internal/framework/plugin"

// init 把本模块登记为插件。ssh-conns 的后端路由随 portfwd 模块挂载，
// 此处仅以独立插件占位以对齐前后端插件 ID。
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
