package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"chaos-go/internal/framework/crud"
)

var ErrDBUnavailable = errors.New("project: database unavailable")

var (
	ErrProjectNotFound = errors.New("project: 项目不存在")
	ErrGroupNotFound   = errors.New("project: 项目组不存在")

	ErrInvalidPath = errors.New("project: 无效的项目路径")
)

type MoveProjectRequest struct {
	TargetGroupID      int    `json:"TargetGroupID"`
	TargetRelativePath string `json:"TargetRelativePath"`
	TargetAbsPath      string `json:"TargetAbsPath"`
}

type ProjectGroup struct {
	crud.BaseModel
	Name         string  `json:"Name"`
	OrderNum     int     `json:"OrderNum" gorm:"default:0"`
	AbsolutePath string  `json:"AbsolutePath"`
	Remark       *string `json:"Remark"`
}

func (ProjectGroup) TableName() string { return "project_groups" }

type Project struct {
	crud.BaseModel
	GroupID        int        `json:"GroupID"`
	Name           string     `json:"Name"`
	AbsolutePath   string     `json:"AbsolutePath"`
	RelativePath   string     `json:"RelativePath"`
	GitURL         *string    `json:"GitURL"`
	Remark         *string    `json:"Remark"`
	LastAccessedAt *time.Time `json:"LastAccessedAt"`
}

func (Project) TableName() string { return "projects" }

type ProjectListItem struct {
	Project
	Claimed bool `json:"Claimed"`
}

func resolveProjectPaths(group ProjectGroup, absolutePath, relativePath string) (string, string, error) {
	var abs, rel string
	var err error
	switch {
	case absolutePath != "":
		abs = filepath.Clean(absolutePath)
		rel, err = filepath.Rel(group.AbsolutePath, abs)
		if err != nil {
			return "", "", fmt.Errorf("计算相对路径失败: %v", err)
		}
	case relativePath != "":
		rel = filepath.Clean(relativePath)
		abs = filepath.Join(group.AbsolutePath, rel)
	default:
		return "", "", fmt.Errorf("必须提供 absolutePath 或 relativePath 之一")
	}
	return abs, rel, nil
}
