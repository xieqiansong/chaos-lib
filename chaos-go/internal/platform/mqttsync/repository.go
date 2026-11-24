package mqttsync

import (
	"log/slog"
	"time"

	"chaos-go/internal/framework/config"
	"chaos-go/pkg/tools"
)

// UpsertMessage 按 node_id + channel + payload 三元组查找：
//   - 已存在 → count+1，更新 msg_id 和 updated_at，返回 (记录, false, nil)
//   - 不存在 → 新建一条 count=1 的记录，返回 (记录, true, nil)
func UpsertMessage(nodeID, channel, payload, msgID string) (MqttSyncMessage, bool, error) {
	db := config.GetDB()
	if db == nil {
		return MqttSyncMessage{}, false, ErrDBUnavailable
	}

	var rec MqttSyncMessage
	now := time.Now()

	// 先查同 node + channel + payload 是否已存在
	err := db.Where("node_id = ? AND channel = ? AND payload = ? AND is_deleted = 0",
		nodeID, channel, payload).First(&rec).Error

	if err == nil {
		// 已存在：count++，更新 msg_id 和时间
		err = db.Model(&rec).Updates(map[string]any{
			"msg_id":    msgID,
			"count":     rec.Count + 1,
			"updated_at": now,
		}).Error
		if err != nil {
			return MqttSyncMessage{}, false, err
		}
		rec.MsgID = msgID
		rec.Count++
		rec.UpdatedAt = now
		return rec, false, nil
	}

	// 不存在：新建
	rec = MqttSyncMessage{
		MsgID:   msgID,
		NodeID:  nodeID,
		Channel: channel,
		Payload: payload,
		Count:   1,
	}
	if err := db.Create(&rec).Error; err != nil {
		return MqttSyncMessage{}, false, err
	}
	return rec, true, nil
}

// saveMessage 供对端消息落库使用，复用 UpsertMessage 去重逻辑
func saveMessage(m wireMessage) (MqttSyncMessage, error) {
	rec, _, err := UpsertMessage(m.NodeID, m.Channel, m.Payload, m.ID)
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
		Order("updated_at DESC").
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
