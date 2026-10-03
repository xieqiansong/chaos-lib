package standarddata

import (
	"errors"

	"chaos-go/internal/config"

	"gorm.io/gorm"
)

// FindActiveByID 按 ID 加载未删除的记录；不存在返回 ErrRecordNotFound。
func FindActiveByID(id string) (*StandardData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var row StandardData
	if err := db.Where("is_deleted = ?", false).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &row, nil
}

// SetStatus 切换指定记录的状态（enabled 字段），返回更新后的完整行。
func SetStatus(id string, status bool) (*StandardData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	if err := db.Model(&StandardData{}).
		Where("is_deleted = ?", false).Where("id = ?", id).
		Update("enabled", status).Error; err != nil {
		return nil, err
	}
	var row StandardData
	if err := db.Where("is_deleted = ?", false).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
