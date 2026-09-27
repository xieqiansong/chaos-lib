// Package datacache 是「数据缓存」：参考 standarddata 的纯 CRUD 基准范式，
// 提供键值型缓存条目的入库管理（类别 / Key / Value / 过期时间 / 压缩算法）。
//
// 压缩算法目前仅做字典预留（列与前端下拉），暂无实际压缩/解压逻辑；
// 后续接入真实压缩时在本业务包内实现，不污染通用 crud 基线。
package datacache

import (
	"time"

	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
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

// Register 把本资源的标准 CRUD 路由挂载到给定路由组。
// 仅纯 CRUD，无扩展子路由，全部交给通用 crud 基线。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "dataCache", &DataCache{}, crud.Opts{
		// Value 为二进制列，无法做文本 LIKE 搜索，故仅 category / key / data_type 可搜。
		Searchable: []string{"category", "key", "data_type"},
		Sortable:   []string{"id", "value_len", "created_at"},
	})
}