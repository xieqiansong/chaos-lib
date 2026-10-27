package mqttsync

import (
	"chaos-go/pkg/tools"
	"errors"
	"log/slog"
	"time"

	"chaos-go/internal/framework/config"
)

func PrepareMessage(m *MqttSyncMessage) error {
	if m.MsgID == "" {
		m.MsgID = tools.UUID()
	}
	if m.NodeID == "" {
		m.NodeID = NodeID()
	}
	if m.Channel == "" {
		m.Channel = defaultChannel
	}
	return nil
}

func BroadcastMessage(m *MqttSyncMessage) error {
	if !config.GetConfig().Mqtt.Enabled {
		return errors.New("MQTT 未启用，无法发送")
	}
	wm := wireMessage{
		ID:      m.MsgID,
		NodeID:  m.NodeID,
		Channel: m.Channel,
		Payload: m.Payload,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
	if err := Publish(wm); err != nil {
		slog.Warn("MQTT 消息广播失败（本地已落库）", "channel", m.Channel, "err", err)
	}
	return nil
}

func Status() StatusInfo {
	cfg := config.GetConfig().Mqtt
	return StatusInfo{
		Enabled:   cfg.Enabled,
		Connected: IsConnected(),
		Broker:    cfg.Broker,
		Prefix:    cfg.Prefix,
		NodeID:    NodeID(),
		Encrypt:   EffectiveEncrypt(),
	}
}

func DeleteChannel(name string) (int64, error) {
	return DeleteChannelByName(name)
}

func ListLatest(prefix string) ([]MessageDTO, error) {
	msgs, err := ListLatestPerChannelLike(prefix)
	if err != nil {
		return nil, err
	}
	dtos := make([]MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dtos = append(dtos, toDTO(m))
	}
	return dtos, nil
}
