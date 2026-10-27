package datacache

import (
	"time"

	"chaos-go/internal/framework/crud"
)

type DataCache struct {
	crud.BaseModel
	Category    string     `json:"Category"`
	Key         string     `json:"Key"`
	Value       []byte     `json:"Value"`
	ExpireAt    *time.Time `json:"ExpireAt"`
	Compression string     `json:"Compression"`
	DataType    string     `json:"DataType"`
	ValueLen    int        `json:"ValueLen" gorm:"default:0"`
	ValueMd5    string     `json:"ValueMd5"`
}

func (DataCache) TableName() string { return "data_caches" }
