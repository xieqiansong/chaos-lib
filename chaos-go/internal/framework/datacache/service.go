package datacache

import (
	"crypto/md5"
	"encoding/hex"
	"time"
)

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
	return CreateCache(row)
}

// Get 提供程序内部调用的缓存读取：返回 (category, key) 下严格最新一条记录
// （id 倒序取首条；过期与否均返回，是否过期由调用方据 ExpireAt 自行判断）。
// 未命中返回 gorm.ErrRecordNotFound。
func Get(category, key string) (*DataCache, error) {
	return FindLatest(category, key)
}
