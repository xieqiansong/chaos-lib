package project

import (
	"errors"
	"time"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

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
