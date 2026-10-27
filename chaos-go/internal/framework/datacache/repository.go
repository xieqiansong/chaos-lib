package datacache

import "chaos-go/internal/framework/config"

func CreateCache(row *DataCache) error {
	return config.GetDB().Create(row).Error
}

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
