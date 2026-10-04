// Package quickedit 管理受管控文件的编辑与快照：登记文件 → 编辑保存（自动快照）→ 回滚到任意历史版本。
// 环境变量以「虚拟文件」形式接入（见 envvar 包注入的读写回调）。
//
// 分层（见 chaos-lib/AGENTS.md「分层契约」）：
//   - model.go：实体 + 领域错误 + 虚拟文件常量与回调
//   - repository.go：数据访问
//   - storage.go：文件内容读写（磁盘 / 虚拟文件分派）
//   - service.go：用例编排
//   - dto.go：响应契约与组装
//   - handler.go：参数解析 + 状态码映射 + 路由注册
package quickedit

import (
	"errors"
	"time"
)

// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
var ErrDBUnavailable = errors.New("quickedit: database unavailable")

// 领域错误哨兵：调用方据此映射 HTTP 状态码。
var (
	ErrFileNotFound     = errors.New("quickedit: 文件不存在")
	ErrSnapshotNotFound = errors.New("quickedit: 快照不存在")
)

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

// ── 环境变量虚拟文件常量 ─────────────────────────────────────────

const (
	EnvVirtualFilePath = "__env__all__"
	EnvVirtualFileName = "环境变量"
	EnvVirtualRemark   = "系统 + 用户环境变量快照（TOML 格式）"
)

// EnvReader 读取环境变量内容（回调 — 避免循环依赖 envvar）。
// 由 internal/app 在启动时注入 envvar.ReadVirtualContent / WriteVirtualContent。
var (
	EnvReadContent  func() (content string, sizeBytes int, err error)
	EnvWriteContent func(content string) (warnings []string, err error)
)

// isEnvVirtualFile 判断是否为「环境变量」虚拟文件（其 filePath 为哨兵值）。
func isEnvVirtualFile(filePath string) bool {
	return filePath == EnvVirtualFilePath
}
