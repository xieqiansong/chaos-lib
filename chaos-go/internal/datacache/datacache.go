// Package datacache 是「数据缓存」：参考 standarddata 的纯 CRUD 基准范式，
// 提供键值型缓存条目的入库管理（类别 / Key / Value / 过期时间 / 压缩算法）。
//
// 压缩算法目前仅做字典预留（列与前端下拉），暂无实际压缩/解压逻辑；
// 后续接入真实压缩时在本业务包内实现，不污染通用 crud 基线。
package datacache

import (
	"crypto/md5"
	"encoding/hex"
	"time"

	"chaos-go/internal/config"
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
	crud.Register[DataCache](rg, "dataCache", crud.Opts[DataCache]{
		// Value 为二进制列，无法做文本 LIKE 搜索，故仅 category / key / data_type 可搜。
		Searchable: []string{"category", "key", "data_type"},
		Sortable:   []string{"id", "value_len", "created_at"},
	})
}

// Set 提供程序内部调用的缓存写入：按 (category, key) 追加一条新记录（不覆盖历史）。
// dataType / compression 为空时分别回退 text / none；ValueLen、ValueMd5 在写入时自动计算。
func Set(category, key string, value []byte, dataType, compression string, expireAt *time.Time) error {
	if dataType == "" {
		dataType = "text"
	}
	if compression == "" {
		compression = "none"
	}
	sum := md5.Sum(value)
	row := &DataCache{
		Category:    category,
		Key:         key,
		Value:       value,
		ExpireAt:    expireAt,
		Compression: compression,
		DataType:    dataType,
		ValueLen:    len(value),
		ValueMd5:    hex.EncodeToString(sum[:]),
	}
	return config.GetDB().Create(row).Error
}

// Get 提供程序内部调用的缓存读取：返回 (category, key) 下严格最新一条记录
// （id 倒序取首条；过期与否均返回，是否过期由调用方据 ExpireAt 自行判断）。
// 未命中返回 gorm.ErrRecordNotFound。
func Get(category, key string) (*DataCache, error) {
	var row DataCache
	err := config.GetDB().
		Where("is_deleted = ?", false).
		Where("category = ? AND key = ?", category, key).
		Order("id DESC").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}