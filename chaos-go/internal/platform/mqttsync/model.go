// Package mqttsync 基于 MQTT Broker 的多节点消息广播：消息落库 + 集群同步（可选 AES 加密）。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：实体与线上报文结构
//   - repository.go：数据访问（只进出模型）
//   - client.go：MQTT 连接 / 加解密 / 发布订阅（基础设施）
//   - service.go：用例（发送、状态、按主题删除、列表视图）
//   - dto.go：响应契约与组装
//   - handler.go：参数解析 + 状态码映射 + 路由注册
package mqttsync

import (
	"errors"
	"time"

	"chaos-go/internal/framework/crud"
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
	NodeID  string `gorm:"size:64" json:"NodeID"`            // 发送方节点标识
	Channel string `gorm:"size:64" json:"Channel"`           // topic（不含前缀），如 broadcast
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

// wireMessage 是走 MQTT 的业务报文（明文 JSON）。传输层由 client 的 seal/open
// 在 MQTT_ENCRYPT=true 时加密为 AES-256-GCM 信封；本地数据库仍存明文业务 JSON。
type wireMessage struct {
	ID      string `json:"id"`
	NodeID  string `json:"node_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Ts      string `json:"ts"`
}
