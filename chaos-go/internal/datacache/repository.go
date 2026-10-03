package datacache

import "chaos-go/internal/config"

// CreateCache 写入一条缓存记录。
func CreateCache(row *DataCache) error {
	return config.GetDB().Create(row).Error
}

// FindLatest 返回 (category, key) 下严格最新一条记录（id 倒序取首条）。
// 未命中返回 gorm.ErrRecordNotFound。
func FindLatest(category, key string) (*DataCache, error) {
	var row DataCache
	err := config.GetDB().
		Where("category = ? AND key = ?", category, key).
		Order("id DESC").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}
