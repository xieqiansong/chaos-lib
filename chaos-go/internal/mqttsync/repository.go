package mqttsync

import (
	"log/slog"

	"chaos-go/internal/config"
)

// saveMessage 按 MsgID 去重写入数据库（已存在则跳过）；返回落库后的记录。
// 发布路径（本机先落库）与订阅路径（对端消息落库）共用，保证不重复。
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
	// FirstOrCreate 基于 MsgID 唯一索引去重，防止 QoS 重投重复落库。
	err := db.Where(MqttSyncMessage{MsgID: m.ID}).
		Attrs(MqttSyncMessage{NodeID: m.NodeID, Channel: m.Channel, Payload: m.Payload}).
		FirstOrCreate(&rec).Error
	return rec, err
}

// ListLatestPerChannel 返回每个 channel 的最新一条消息（按 created_at 倒序），无分页。
// 旧消息全部保留在数据库，仅界面展示最新一条；DTO 转换见 service.ListLatest。
func ListLatestPerChannel() ([]MqttSyncMessage, error) {
	return ListLatestPerChannelLike("")
}

// ListLatestPerChannelLike 返回 channel 匹配指定前缀的每个 channel 最新一条消息。
// prefix 为空时等价于 ListLatestPerChannel（全部 channel）。
func ListLatestPerChannelLike(prefix string) ([]MqttSyncMessage, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	sub := db.Table("mqtt_sync_messages").
		Select("MAX(id)").
		Where("is_deleted = ?", false).
		Group("channel")
	if prefix != "" {
		sub = sub.Where("channel LIKE ?", prefix+"%")
	}
	var msgs []MqttSyncMessage
	if err := db.
		Where("id IN (?)", sub).
		Where("is_deleted = ?", false).
		Order("created_at DESC").
		Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

// DeleteChannelByName 将指定主题下的全部消息软删除（IsDeleted = true），返回受影响行数。
func DeleteChannelByName(name string) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	res := db.Model(&MqttSyncMessage{}).
		Where("channel = ? AND is_deleted = ?", name, false).
		Update("is_deleted", true)
	return res.RowsAffected, res.Error
}

// GetOrCreateNodeID 从 mqtt_sync_node 表加载节点标识；不存在则生成并持久化，返回该标识。
// 数据库不可用时返回 ErrDBUnavailable；持久化失败仅警告并返回内存态 id，不阻断启动。
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
	id := newID()
	rec := MqttSyncNode{NodeID: id}
	if err2 := db.Create(&rec).Error; err2 != nil {
		slog.Warn("MQTT 节点标识持久化失败，使用内存态", "err", err2)
	}
	return id, nil
}
