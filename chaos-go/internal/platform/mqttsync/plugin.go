package mqttsync

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

// init 把本模块登记为插件。原先 handler.go 中的 routehub.RegisterV1 调用已移除。
// 注意：mqttsync.Start() 由 app.go 在后台 goroutine 中调用（未启用 / broker 或数据库不可达均不阻塞启动），
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
