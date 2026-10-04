package standarddata

import (
	"errors"
	"time"

	"chaos-go/internal/framework/crud"
)

// ErrDBUnavailable 表示数据库单例不可用。
var ErrDBUnavailable = errors.New("standarddata: database unavailable")

// ErrRecordNotFound 指定记录不存在（或已软删）。
var ErrRecordNotFound = errors.New("standarddata: 记录不存在")

// StandardData 标准数据行。
// 嵌入 crud.BaseModel 自动获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type StandardData struct {
	crud.BaseModel
	Name        string     `json:"Name"`
	Code        string     `json:"Code"`
	Description string     `json:"Description"`
	Category    string     `json:"Category"`
	Quantity    int        `json:"Quantity"`
	Price       float64    `json:"Price"`
	Enabled     bool       `json:"Enabled"`
	Config      string     `json:"Config"` // 扩展 JSON（以文本存储，展示 json 类型列）
	EffectiveAt *time.Time `json:"EffectiveAt"`
	Sort        int        `json:"Sort" gorm:"default:0"`
}

// TableName 显式指定表名（与前端资源名 standardData 对应）。
func (StandardData) TableName() string { return "standard_datas" }
