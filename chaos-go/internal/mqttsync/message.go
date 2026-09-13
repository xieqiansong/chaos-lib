package mqttsync

import (
	"log/slog"
	"time"

	"chaos-go/config"
	"github.com/gin-gonic/gin"
)

// MqttSyncMessage 是订阅/发布落库的消息记录。
// 列名由 GORM 自动推断为 snake_case，不加 column 标签；软删除用 IsDeleted + 手动过滤。
type MqttSyncMessage struct {
	ID        uint      `gorm:"primaryKey"`
	MsgID     string    `gorm:"uniqueIndex;size:64"` // 线上 JSON 的 id，全局唯一，用于去重
	NodeID    string    `gorm:"size:64"`             // 发送方节点标识
	Channel   string    `gorm:"size:64"`             // topic（不含前缀），如 broadcast
	Payload   string    `gorm:"type:text"`
	CreatedAt time.Time
	IsDeleted bool `gorm:"default:false"`
}

// MqttSyncNode 保存本机节点标识；NodeID 在首次启动时生成并持久化，全程稳定。
type MqttSyncNode struct {
	ID        uint      `gorm:"primaryKey"`
	NodeID    string    `gorm:"uniqueIndex;size:64"`
	CreatedAt time.Time
}

// MessageDTO 是返回给前端的消息视图，附带 is_self 便于界面区分本机消息。
type MessageDTO struct {
	MsgID     string    `json:"msg_id"`
	NodeID    string    `json:"node_id"`
	Channel   string    `json:"channel"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
	IsSelf    bool      `json:"is_self"`
}

// wireMessage 是走 MQTT 的线上报文（明文 JSON，加密为后续变更）。
type wireMessage struct {
	ID      string `json:"id"`
	NodeID  string `json:"node_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Ts      string `json:"ts"`
}

func toDTO(m MqttSyncMessage) MessageDTO {
	return MessageDTO{
		MsgID:     m.MsgID,
		NodeID:    m.NodeID,
		Channel:   m.Channel,
		Payload:   m.Payload,
		CreatedAt: m.CreatedAt,
		IsSelf:    m.NodeID != "" && m.NodeID == NodeID(),
	}
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
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var msgs []MqttSyncMessage
	err := db.
		Where("id IN (?)", db.Table("mqtt_sync_messages").
			Select("MAX(id)").
			Where("is_deleted = ?", false).
			Group("channel")).
		Where("is_deleted = ?", false).
		Order("created_at DESC").
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	dtos := make([]MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dtos = append(dtos, toDTO(m))
	}
	return dtos, nil
}

// SendMessage 处理 POST /api/mqttSync/messages：本地落库 + 尽力广播。
func SendMessage(c *gin.Context) {
	cfg := config.GetConfig().Mqtt
	if !cfg.Enabled {
		c.JSON(400, gin.H{"error": "mqtt sync disabled"})
		return
	}
	var req struct {
		Channel string `json:"channel"`
		Payload string `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Payload == "" {
		c.JSON(400, gin.H{"error": "payload is empty"})
		return
	}
	channel := req.Channel
	if channel == "" {
		channel = defaultChannel
	}

	m := wireMessage{
		ID:      newID(),
		NodeID:  NodeID(),
		Channel: channel,
		Payload: req.Payload,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
	rec, err := saveMessage(m) // 本机即时落库
	if err != nil {
		slog.Error("MQTT 消息本地落库失败", "err", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if pubErr := Publish(m); pubErr != nil {
		// broker 离线：本地已落库，仅记日志，不回滚、不报失败给调用方造成「丢失」错觉
		slog.Warn("MQTT 发布失败（本地已落库）", "channel", channel, "err", pubErr)
	}
	c.JSON(200, toDTO(rec))
}

// ListMessages 处理 GET /api/mqttSync/messages：每个 topic 仅返回最新一条。
func ListMessages(c *gin.Context) {
	msgs, err := ListLatestPerChannel()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, msgs)
}

// Status 处理 GET /api/mqttSync/status：暴露启用 / 连接状态与节点标识。
func Status(c *gin.Context) {
	cfg := config.GetConfig().Mqtt
	c.JSON(200, gin.H{
		"enabled":  cfg.Enabled,
		"connected": IsConnected(),
		"broker":   cfg.Broker,
		"prefix":   cfg.Prefix,
		"node_id":  NodeID(),
	})
}
