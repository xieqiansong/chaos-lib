package sdk

import (
	"errors"

	"chaos-go/internal/config"
	"gorm.io/gorm"
)

// ListActiveSdkSources 返回所有未删除的 SDK 类型。
func ListActiveSdkSources() ([]SdkSource, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var srcs []SdkSource
	if err := db.Where("is_deleted = ?", false).Find(&srcs).Error; err != nil {
		return nil, err
	}
	return srcs, nil
}

// FindSdkSource 按类型名加载未删除的 SDK 来源定义；不存在返回 ErrSdkNotFound。
func FindSdkSource(typ string) (*SdkSource, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var s SdkSource
	if err := db.Where("name = ? AND is_deleted = ?", typ, false).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSdkNotFound
		}
		return nil, err
	}
	return &s, nil
}

// SetCurrentVersion 持久化某 SDK 类型当前启用的版本绝对路径。
func SetCurrentVersion(typ string, abs string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", typ, false).
		Update("current", abs).Error
}

// CountActiveByName 统计同名未删除的 SDK 类型数量（用于唯一性校验）。
func CountActiveByName(name string) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var count int64
	if err := db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", name, false).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateSdkSourceRow 新增一条 SDK 类型记录。
func CreateSdkSourceRow(s *SdkSource) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(s).Error
}

// ApplySdkSourcePatch 按字段映射更新 SDK 类型（Sources/Current/Enabled/Note）。
func ApplySdkSourcePatch(name string, updates map[string]interface{}) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", name, false).
		Updates(updates).Error
}

// SoftDeleteSdkSource 软删除某 SDK 类型。
func SoftDeleteSdkSource(name string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", name, false).
		Update("is_deleted", true).Error
}
