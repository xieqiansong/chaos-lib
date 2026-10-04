package filelink

import (
	"errors"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

// ErrLinkNotFound 指定联接点记录不存在（或已软删）。
var ErrLinkNotFound = errors.New("filelink: 文件连接不存在")

// findActiveByID 按 ID 加载未软删的联接点记录。
func findActiveByID(id int) (*FileLink, error) {
	var link FileLink
	if err := config.GetDB().First(&link, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}
	return &link, nil
}

// updateStatus 持久化联接点的启用状态。
func updateStatus(id int, status bool) error {
	return config.GetDB().Model(&FileLink{}).Where("id = ?", id).Update("status", status).Error
}
