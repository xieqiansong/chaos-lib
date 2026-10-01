package mqttsync

import (
	"errors"
	"time"

	"chaos-go/internal/crud"
	"gorm.io/gorm"
)

// 包级错误哨兵。
var (
	// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
	ErrDBUnavailable = errors.New("mqttsync: database unavailable")
	// ErrNotConnected 表示 broker 当前未连接，消息仅本地落库。
	ErrNotConnected = errors.New("mqttsync: broker not connected")
)

// defaultChannel 是缺省主题（topic 不含前缀）。
const defaultChannel = "broadcast"

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
func toMessageDTOs(rows []*MqttSyncMessage) any {
	out := make([]MessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toDTO(*m))
	}
	return out
}

// wireMessage 是走 MQTT 的业务报文（明文 JSON）。传输层由 service 的 seal/open
// 在 MQTT_ENCRYPT=true 时加密为 AES-256-GCM 信封；本地数据库仍存明文业务 JSON。
type wireMessage struct {
	ID      string `json:"id"`
	NodeID  string `json:"node_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Ts      string `json:"ts"`
}
