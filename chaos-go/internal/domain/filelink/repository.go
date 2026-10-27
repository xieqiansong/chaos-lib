package filelink

import (
	"errors"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

var ErrLinkNotFound = errors.New("filelink: 文件连接不存在")

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

func updateStatus(id int, status bool) error {
	return config.GetDB().Model(&FileLink{}).Where("id = ?", id).Update("status", status).Error
}
