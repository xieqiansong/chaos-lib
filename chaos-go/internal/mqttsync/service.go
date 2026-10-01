package mqttsync

import (
	"errors"
	"log/slog"
	"time"

	"chaos-go/internal/config"
)

// ── 用例 ────────────────────────────────────────────────────────

// PrepareMessage 落库前补齐 MsgID / NodeID / Channel：
// 与广播报文保持一致，使对端按 MsgID 去重、本机回声按 NodeID 丢弃。
func PrepareMessage(m *MqttSyncMessage) error {
	if m.MsgID == "" {
		m.MsgID = newID()
	}
	if m.NodeID == "" {
		m.NodeID = NodeID()
	}
	if m.Channel == "" {
		m.Channel = defaultChannel
	}
	return nil
}

// BroadcastMessage 落库后向集群广播（与落库共用同一 MsgID，便于对端去重）。
// 通道未启用视为失败（回滚，避免「本地已存却没广播」的误导）；
// broker 离线则仅记日志——本地已落库，与发布语义一致。
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

// Status 返回启用 / 连接状态与节点标识。
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

// DeleteChannel 将指定主题下的全部消息软删除，返回受影响行数。
func DeleteChannel(name string) (int64, error) {
	return DeleteChannelByName(name)
}

// ListLatest 返回 channel 匹配指定前缀的最新一条消息（每个 channel 一条）。
// prefix 为空时等价于全部 channel。
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
