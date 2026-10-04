package datacache

import (
	"time"

	"chaos-go/internal/framework/crud"
)

// DataCache 数据缓存行。
// 嵌入 crud.BaseModel 自动获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type DataCache struct {
	crud.BaseModel
	Category    string     `json:"Category"`
	Key         string     `json:"Key"`         // 缓存键（列名 key 由 GORM 自动加引号，规避保留字）
	Value       []byte     `json:"Value"`       // 缓存值（二进制 BLOB/BYTEA，可存任意原始字节；JSON 传输为 base64）
	ExpireAt    *time.Time `json:"ExpireAt"`    // 过期时间（可为空，表示永不过期）
	Compression string     `json:"Compression"` // 压缩算法（预留字典：none / gzip / zstd）
	DataType    string     `json:"DataType"`    // 元素数据类型（如 text/json/png/mp4，供统计用；表单手动选择）
	ValueLen    int        `json:"ValueLen" gorm:"default:0"` // 值字节长度（手动维护，供统计用）
	ValueMd5    string     `json:"ValueMd5"`    // 值 MD5（手动维护，供去重/统计用）
}

// TableName 显式指定表名（与前端资源名 dataCache 对应）。
func (DataCache) TableName() string { return "data_caches" }
