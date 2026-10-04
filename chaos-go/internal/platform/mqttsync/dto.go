package mqttsync

import "time"

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

// StatusInfo 连接状态视图。
type StatusInfo struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Broker    string `json:"broker"`
	Prefix    string `json:"prefix"`
	NodeID    string `json:"node_id"`
	Encrypt   bool   `json:"encrypt"`
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
