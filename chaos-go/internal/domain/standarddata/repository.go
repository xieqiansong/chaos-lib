package standarddata

import (
	"errors"

	"chaos-go/internal/framework/config"

	"gorm.io/gorm"
)

func FindActiveByID(id string) (*StandardData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var row StandardData
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &row, nil
}

func SetStatus(id string, status bool) (*StandardData, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	if err := db.Model(&StandardData{}).
		Where("id = ?", id).
		Update("enabled", status).Error; err != nil {
		return nil, err
	}
	var row StandardData
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
