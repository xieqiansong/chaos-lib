package project

import (
	"errors"
	"time"

	"chaos-go/internal/config"
	"gorm.io/gorm"
)

// FindGroupByID 按 ID 加载未软删的项目组；不存在返回 ErrGroupNotFound。
func FindGroupByID(id int) (*ProjectGroup, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var g ProjectGroup
	if err := db.Where("id = ?", id).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &g, nil
}

// FindProjectByID 按 ID 加载未软删的项目；不存在返回 ErrProjectNotFound。
func FindProjectByID(id int) (*Project, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var p Project
	if err := db.Where("id = ?", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CascadeDeleteProjectsByGroup 删组（软删）后级联软删其子项目。
func CascadeDeleteProjectsByGroup(groupID int) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	now := time.Now()
	return db.Model(&Project{}).
		Where("group_id = ?", groupID).
		Updates(map[string]interface{}{"is_deleted": 1, "updated_at": now}).Error
}

// FindActiveProjectsByGroup 返回某项目组下所有未软删的子项目。
func FindActiveProjectsByGroup(groupID int) ([]Project, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var children []Project
	if err := db.Where("group_id = ?", groupID).Find(&children).Error; err != nil {
		return nil, err
	}
	return children, nil
}

// UpdateProjectAbsolutePath 组根目录变更后重算并写入某子项目的绝对路径。
func UpdateProjectAbsolutePath(id int, newAbs string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	now := time.Now()
	return db.Model(&Project{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"absolute_path": newAbs, "updated_at": now}).Error
}

// ListActiveProjects 按 groupId / name 过滤，返回未软删项目（按访问时间倒序）。
func ListActiveProjects(groupID *int, name string) ([]Project, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	q := db.Model(&Project{})
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	var projects []Project
	if err := q.Order("last_accessed_at DESC NULLS LAST, created_at DESC, id DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// UpdateProjectLocation 移动项目后写回 group_id / 绝对路径 / 相对路径。
func UpdateProjectLocation(id, groupID int, newAbs, newRel string) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	now := time.Now()
	return db.Model(&Project{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"group_id":      groupID,
			"absolute_path": newAbs,
			"relative_path": newRel,
			"updated_at":    now,
		}).Error
}

// TouchProjectAccess 记录项目访问时间。
func TouchProjectAccess(id int) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	now := time.Now()
	return db.Model(&Project{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"last_accessed_at": now, "updated_at": now}).Error
}
