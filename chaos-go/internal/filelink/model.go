// Package filelink 管理本地文件/目录联接点（Windows junction / POSIX symlink）。
//
// 参考标准数据（internal/standarddata）实现：纯 CRUD 交给通用 crud，业务特有的部分用回调注入：
//   - ToResponse：派生字段 LinkStatus 按文件系统实时计算（不落库，返回时重新算）
//   - AfterCreate：校验源/目标路径，不合法则不落库
//   - AfterDelete：清理联接点，清理失败则整笔软删回滚
//
// 启用开关有副作用（建/删联接点）且带校验，故保留自定义子路由 PATCH /fileLinks/:id/status，
// 与标准数据保持同一套「标准之下的自定义」范式。
package filelink

import (
	"time"

	"chaos-go/internal/crud"
)

// FileLink 文件连接。嵌入 crud.BaseModel 获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type FileLink struct {
	crud.BaseModel
	SourcePath string `json:"SourcePath"`
	TargetPath string `json:"TargetPath"`
	Status     bool   `json:"Status"`
	Remark     string `json:"Remark"`
	Sort       int    `gorm:"default:0" json:"Sort"`
}

// TableName 显式指定表名。
func (FileLink) TableName() string { return "file_links" }

// FileLinkResponse 对外响应：模型字段之外附带实时计算的 LinkStatus。
type FileLinkResponse struct {
	ID         int       `json:"ID"`
	SourcePath string    `json:"SourcePath"`
	TargetPath string    `json:"TargetPath"`
	Status     bool      `json:"Status"`
	Remark     string    `json:"Remark"`
	Sort       int       `json:"Sort"`
	LinkStatus string    `json:"LinkStatus"`
	CreatedAt  time.Time `json:"CreatedAt"`
	UpdatedAt  time.Time `json:"UpdatedAt"`
}
