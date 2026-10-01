package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"chaos-go/internal/crud"
)

// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
var ErrDBUnavailable = errors.New("project: database unavailable")

// 领域错误哨兵：调用方据此映射 HTTP 状态码（NotFound → 404，InvalidPath → 400）。
var (
	ErrProjectNotFound = errors.New("project: 项目不存在")
	ErrGroupNotFound   = errors.New("project: 项目组不存在")
	// ErrInvalidPath 路径推导失败：未提供绝对路径 / 相对路径，或相对路径无法计算。
	ErrInvalidPath = errors.New("project: 无效的项目路径")
)

// MoveProjectRequest 移动项目的请求体。
type MoveProjectRequest struct {
	TargetGroupID      int    `json:"TargetGroupID"`
	TargetRelativePath string `json:"TargetRelativePath"`
	TargetAbsPath      string `json:"TargetAbsPath"`
}

// ProjectGroup 项目组：拥有一个根目录（AbsolutePath），其下项目通过 RelativePath 相对该根目录定位。
type ProjectGroup struct {
	crud.BaseModel
	Name         string  `json:"Name"`
	OrderNum     int     `json:"OrderNum" gorm:"default:0"`
	AbsolutePath string  `json:"AbsolutePath"`
	Remark       *string `json:"Remark"`
}

func (ProjectGroup) TableName() string { return "project_groups" }

// Project 项目：绝对路径 = 所属项目组绝对路径 + 相对路径。
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

// ProjectListItem 项目列表项：指定 groupId 时合并磁盘未认领目录。
//   - Claimed=true：已入库
//   - Claimed=false：仅存于磁盘、尚未认领；其 ID 为负的哨兵值（避免前端 DataTable 行 key 冲突），其余为空
type ProjectListItem struct {
	Project
	Claimed bool `json:"Claimed"`
}

// resolveProjectPaths 由项目组根目录与 绝对路径/相对路径 之一推导出另一个，返回 (绝对路径, 相对路径)。
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
