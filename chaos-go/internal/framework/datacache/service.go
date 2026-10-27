package datacache

import (
	"crypto/md5"
	"encoding/hex"
	"time"
)

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

func Get(category, key string) (*DataCache, error) {
	return FindLatest(category, key)
}
