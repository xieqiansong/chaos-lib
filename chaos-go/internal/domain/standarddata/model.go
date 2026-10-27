package standarddata

import (
	"errors"
	"time"

	"chaos-go/internal/framework/crud"
)

var ErrDBUnavailable = errors.New("standarddata: database unavailable")

var ErrRecordNotFound = errors.New("standarddata: 记录不存在")

type StandardData struct {
	crud.BaseModel
	Name        string     `json:"Name"`
	Code        string     `json:"Code"`
	Description string     `json:"Description"`
	Category    string     `json:"Category"`
	Quantity    int        `json:"Quantity"`
	Price       float64    `json:"Price"`
	Enabled     bool       `json:"Enabled"`
	Config      string     `json:"Config"`
	EffectiveAt *time.Time `json:"EffectiveAt"`
	Sort        int        `json:"Sort" gorm:"default:0"`
}

func (StandardData) TableName() string { return "standard_datas" }
