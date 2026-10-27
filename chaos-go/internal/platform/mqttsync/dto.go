package mqttsync

import "time"

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

type StatusInfo struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Broker    string `json:"broker"`
	Prefix    string `json:"prefix"`
	NodeID    string `json:"node_id"`
	Encrypt   bool   `json:"encrypt"`
}

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

func toMessageDTOs(rows []*MqttSyncMessage) any {
	out := make([]MessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toDTO(*m))
	}
	return out
}
