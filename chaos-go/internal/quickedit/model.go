package quickedit

import (
	"errors"
	"time"
)

// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
var ErrDBUnavailable = errors.New("quickedit: database unavailable")

// QuickEditFile 受管控的文件记录。
type QuickEditFile struct {
	ID        int       `gorm:"primaryKey"`
	Name      string    ``
	FilePath  string    `gorm:"uniqueIndex:idx_quick_edit_files_path"`
	Remark    string    ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

// QuickEditSnapshot 文件内容的历史快照（每次保存/更新/回滚都会落一条）。
type QuickEditSnapshot struct {
	ID        int       `gorm:"primaryKey"`
	FileID    int       ``
	Content   string    ``
	SizeBytes int       ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

// QuickEditFileResponse 文件列表/详情响应（附带最近一次快照信息）。
type QuickEditFileResponse struct {
	ID               int
	Name             string
	FilePath         string
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastSnapshotID   int
	LastSnapshotTime time.Time
}

// QuickEditSnapshotResponse 快照列表项。
type QuickEditSnapshotResponse struct {
	ID        int
	FileID    int
	SizeBytes int
	CreatedAt time.Time
}

// ── 环境变量虚拟文件常量 ─────────────────────────────────────────

const (
	EnvVirtualFilePath = "__env__all__"
	EnvVirtualFileName = "环境变量"
	EnvVirtualRemark   = "系统 + 用户环境变量快照（TOML 格式）"
)

// EnvReader 读取环境变量内容（回调 — 避免循环依赖 envvar）。
// 由 main.go 在启动时注入 envvar.ReadVirtualContent / WriteVirtualContent。
var (
	EnvReadContent  func() (content string, sizeBytes int, err error)
	EnvWriteContent func(content string) (warnings []string, err error)
)

// isEnvVirtualFile 判断是否为「环境变量」虚拟文件（其 filePath 为哨兵值）。
func isEnvVirtualFile(filePath string) bool {
	return filePath == EnvVirtualFilePath
}
