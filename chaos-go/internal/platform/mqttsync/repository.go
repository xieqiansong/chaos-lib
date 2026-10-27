package mqttsync

import (
	"chaos-go/pkg/tools"
	"log/slog"

	"chaos-go/internal/framework/config"
)

func saveMessage(m wireMessage) (MqttSyncMessage, error) {
	db := config.GetDB()
	if db == nil {
		return MqttSyncMessage{}, ErrDBUnavailable
	}
	rec := MqttSyncMessage{
		MsgID:   m.ID,
		NodeID:  m.NodeID,
		Channel: m.Channel,
		Payload: m.Payload,
	}

	err := db.Where(MqttSyncMessage{MsgID: m.ID}).
		Attrs(MqttSyncMessage{NodeID: m.NodeID, Channel: m.Channel, Payload: m.Payload}).
		FirstOrCreate(&rec).Error
	return rec, err
}

func ListLatestPerChannel() ([]MqttSyncMessage, error) {
	return ListLatestPerChannelLike("")
}

func ListLatestPerChannelLike(prefix string) ([]MqttSyncMessage, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}

	sub := db.Table("mqtt_sync_messages").
		Select("MAX(id)").
		Where("is_deleted = ?", 0).
		Group("channel")
	if prefix != "" {
		sub = sub.Where("channel LIKE ?", prefix+"%")
	}
	var msgs []MqttSyncMessage
	if err := db.
		Where("id IN (?)", sub).
		Order("created_at DESC").
		Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func DeleteChannelByName(name string) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}

	res := db.Where("channel = ?", name).Delete(&MqttSyncMessage{})
	return res.RowsAffected, res.Error
}

func GetOrCreateNodeID() (string, error) {
	db := config.GetDB()
	if db == nil {
		return "", ErrDBUnavailable
	}
	var node MqttSyncNode
	err := db.Order("id ASC").First(&node).Error
	if err == nil && node.NodeID != "" {
		return node.NodeID, nil
	}
	id := tools.UUID()
	rec := MqttSyncNode{NodeID: id}
	if err2 := db.Create(&rec).Error; err2 != nil {
		slog.Warn("MQTT 节点标识持久化失败，使用内存态", "err", err2)
	}
	return id, nil
}
