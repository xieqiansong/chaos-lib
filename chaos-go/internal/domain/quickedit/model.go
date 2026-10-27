package quickedit

import (
	"errors"
	"time"
)

var ErrDBUnavailable = errors.New("quickedit: database unavailable")

var (
	ErrFileNotFound     = errors.New("quickedit: 文件不存在")
	ErrSnapshotNotFound = errors.New("quickedit: 快照不存在")
)

type QuickEditFile struct {
	ID        int       `gorm:"primaryKey"`
	Name      string    ``
	FilePath  string    `gorm:"uniqueIndex:idx_quick_edit_files_path"`
	Remark    string    ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type QuickEditSnapshot struct {
	ID        int       `gorm:"primaryKey"`
	FileID    int       ``
	Content   string    ``
	SizeBytes int       ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

const (
	EnvVirtualFilePath = "__env__all__"
	EnvVirtualFileName = "环境变量"
	EnvVirtualRemark   = "系统 + 用户环境变量快照（TOML 格式）"
)

var (
	EnvReadContent  func() (content string, sizeBytes int, err error)
	EnvWriteContent func(content string) (warnings []string, err error)
)

func isEnvVirtualFile(filePath string) bool {
	return filePath == EnvVirtualFilePath
}
