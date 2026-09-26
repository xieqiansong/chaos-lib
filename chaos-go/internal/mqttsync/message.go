package mqttsync

import (
	"time"

	"chaos-go/config"
	"chaos-go/internal/crud"
	"gorm.io/gorm"
)

// MqttSyncMessage 是订阅/发布落库的消息记录。
// 嵌入 crud.BaseModel 获得 ID / CreatedAt / UpdatedAt / IsDeleted；
// 列名由 GORM 自动推断为 snake_case，不加 column 标签；软删除手动过滤。
type MqttSyncMessage struct {
	crud.BaseModel
	MsgID   string `gorm:"uniqueIndex;size:64" json:"MsgID"` // 线上报文 id（uuid），全局唯一，用于去重
	NodeID  string `gorm:"size:64" json:"NodeID"`           // 发送方节点标识
	Channel string `gorm:"size:64" json:"Channel"`          // topic（不含前缀），如 broadcast
	Payload string `gorm:"type:text" json:"Payload"`
}

// MqttSyncNode 保存本机节点标识；NodeID 在首次启动时生成并持久化，全程稳定。
type MqttSyncNode struct {
	ID        uint      `gorm:"primaryKey"`
	NodeID    string    `gorm:"uniqueIndex;size:64" json:"NodeID"`
	CreatedAt time.Time `json:"-"`
}

// TableName 显式指定表名（与 CRUD 前缀 mqttSync 对应）。
func (MqttSyncMessage) TableName() string { return "mqtt_sync_messages" }

// MessageDTO 是返回给前端的消息视图，附带 IsSelf 便于界面区分本机消息。
type MessageDTO struct {
	ID        int       `json:"ID"`
	MsgID     string    `json:"MsgID"`
	NodeID    string    `json:"NodeID"`
	Channel   string    `json:"Channel"`
	Payload   string    `json:"Payload"`
	IsSelf    bool      `json:"IsSelf"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
}

// BeforeCreate 在落库前补齐 MsgID / NodeID / Channel：
// 与广播报文保持一致，使对端按 MsgID 去重、本机回声按 NodeID 丢弃。
func (m *MqttSyncMessage) BeforeCreate(_ *gorm.DB) error {
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

// toDTO 转换为前端视图；IsSelf 依据本机节点标识计算。
func toDTO(m MqttSyncMessage) MessageDTO {
	return MessageDTO{
		ID:        m.ID,
		MsgID:     m.MsgID,
		NodeID:    m.NodeID,
		Channel:   m.Channel,
		Payload:   m.Payload,
		IsSelf:    m.NodeID != "" && m.NodeID == NodeID(),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// toMessageDTOs 是 crud 的 ToResponse 回调：把 []*MqttSyncMessage 整批转为 []MessageDTO。
func toMessageDTOs(rows any) any {
	ptrs, ok := rows.([]*MqttSyncMessage)
	if !ok {
		return rows
	}
	out := make([]MessageDTO, 0, len(ptrs))
	for _, m := range ptrs {
		out = append(out, toDTO(*m))
	}
	return out
}

// wireMessage 是走 MQTT 的业务报文（明文 JSON）。传输层由 mqtt.go 的 seal/open
// 在 MQTT_ENCRYPT=true 时加密为 AES-256-GCM 信封；本地数据库仍存明文业务 JSON。
type wireMessage struct {
	ID      string `json:"id"`
	NodeID  string `json:"node_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Ts      string `json:"ts"`
}

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
// 旧消息全部保留在数据库，仅界面展示最新一条。
func ListLatestPerChannel() ([]MessageDTO, error) {
	return ListLatestPerChannelLike("")
}

// ListLatestPerChannelLike 返回 channel 匹配指定前缀的每个 channel 最新一条消息。
// prefix 为空时等价于 ListLatestPerChannel（全部 channel）。
func ListLatestPerChannelLike(prefix string) ([]MessageDTO, error) {
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
	dtos := make([]MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dtos = append(dtos, toDTO(m))
	}
	return dtos, nil
}
