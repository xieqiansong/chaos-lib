package mqttsync

import (
	"errors"

	"chaos-go/internal/framework/crud"
)

var (
	ErrDBUnavailable = errors.New("mqttsync: database unavailable")

	ErrNotConnected = errors.New("mqttsync: broker not connected")
)

const defaultChannel = "broadcast"

type MqttSyncMessage struct {
	crud.BaseModel
	MsgID   string `gorm:"uniqueIndex;size:64" json:"MsgID"`
	NodeID  string `gorm:"size:64;index" json:"NodeID"`
	Channel string `gorm:"size:64;index" json:"Channel"`
	Payload string `gorm:"type:text" json:"Payload"`
	Count   int    `gorm:"default:1" json:"Count"`
}

func (MqttSyncMessage) TableName() string { return "mqtt_sync_messages" }

type wireMessage struct {
	ID      string `json:"id"`
	NodeID  string `json:"node_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Ts      string `json:"ts"`
}
