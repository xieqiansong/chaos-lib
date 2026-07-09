package models

import "time"

// ProjectGroup 项目组
// 项目组拥有一个根目录（AbsolutePath），其下项目通过 RelativePath 相对该根目录定位。
type ProjectGroup struct {
	ID           int       `gorm:"primaryKey"`
	Name         string    ``
	OrderNum     int       `gorm:"default:0"`
	AbsolutePath string    ``
	Remark       *string   ``
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted    bool      `gorm:"default:false"`
	IsRecycleBin bool      `gorm:"default:false"`
}

func (ProjectGroup) TableName() string {
	return "project_groups"
}

// Project 项目
// 项目的绝对路径由所属项目组的绝对路径与相对路径组合得到：
//
//	AbsolutePath = Group.AbsolutePath + RelativePath
//
// 移动项目文件夹时会同时更新磁盘目录与这里的 GroupID / AbsolutePath / RelativePath。
type Project struct {
	ID             int        `gorm:"primaryKey"`
	GroupID        int        ``
	Name           string     ``
	AbsolutePath   string     ``
	RelativePath   string     ``
	GitURL         *string    ``
	Remark         *string    ``
	LastAccessedAt *time.Time ``
	CreatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted      bool       `gorm:"default:false"`
}

func (Project) TableName() string {
	return "projects"
}

// ProjectListItem 项目列表项。
// 当指定 groupId 时，ListProjects 会扫描组根目录下的子目录并与数据库合并返回。
//   - Claimed=true：已入库（已认领）
//   - Claimed=false：仅存在于磁盘、尚未认领；其 ID 为 0，GitURL/Remark/LastAccessedAt 等为空
type ProjectListItem struct {
	Project
	Claimed bool ``
}
