package mqttsync

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除。
// 注意：mqttsync.Start() 当前在 app.go 中处于注释禁用状态（未启用或 broker 不可达时不阻塞启动），
// 故此处不登记 Start 钩子，保持现有行为不变。
func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "mqttsync",
			Name:         "MQTT同步",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		Models: []any{&MqttSyncMessage{}, &MqttSyncNode{}},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}
