package mqttsync

import (
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/web"
)

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
