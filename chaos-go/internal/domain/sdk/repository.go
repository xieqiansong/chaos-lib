package sdk

import (
	"errors"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

func ListActiveSdkSources() ([]SdkSource, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var srcs []SdkSource
	if err := db.Find(&srcs).Error; err != nil {
		return nil, err
	}
	return srcs, nil
}

func FindSdkSource(typ string) (*SdkSource, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var s SdkSource
	if err := db.Where("name = ?", typ).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSdkNotFound
		}
		return nil, err
	}
	return &s, nil
}

func SetCurrentVersion(typ string, abs string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&SdkSource{}).Where("name = ?", typ).
		Update("current", abs).Error
}

func CountActiveByName(name string) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var count int64
	if err := db.Model(&SdkSource{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func CreateSdkSourceRow(s *SdkSource) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(s).Error
}

func ApplySdkSourcePatch(name string, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&SdkSource{}).Where("name = ?", name).
		Updates(updates).Error
}

func SoftDeleteSdkSource(name string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}

	return db.Where("name = ?", name).Delete(&SdkSource{}).Error
}
