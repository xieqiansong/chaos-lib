// Package note 实现 Web 端 Markdown 笔记能力（浏览目录树 → 列表 → 检索）。
//
// 核心约定（详见 docs/note-module-design.md）：
//   - 磁盘文件是唯一真相，数据库只存派生索引，可随时全量重建；
//   - 分层契约单向向下：handler → service → repository（唯一 DB 出口）；
//   - model.go 不碰任何 IO；路径安全校验用纯函数 resolveSafePath。
package note

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaos-go/internal/framework/crud"
)

// 领域错误哨兵：handler 层据此映射到 HTTP 状态码。
var (
	ErrNoteNotFound     = errors.New("note: 笔记不存在")
	ErrInvalidPath      = errors.New("note: 非法路径")
	ErrPathEscapesVault = errors.New("note: 路径超出笔记库范围")
	ErrAlreadyExists    = errors.New("note: 同名文件已存在")
	ErrConflict         = errors.New("note: 文件已被外部修改")
	ErrContentTooLarge  = errors.New("note: 文件过大，暂不支持在线编辑")
	ErrScanInProgress   = errors.New("note: 扫描正在进行中")
	ErrVaultUnavailable = errors.New("note: 笔记库未配置或不可用")
)

// DefaultVaultID 一期单 vault，固定为 1（表已预留 VaultID 以便 P2 扩展多 vault）。
const DefaultVaultID = 1

// Note 笔记索引项。本表是磁盘 Vault 的派生索引，删除后可通过重新扫描完整重建。
// 嵌入 crud.BaseModel 复用主键 / 时间戳 / soft_delete（flag 模式），与全仓模型保持一致。
type Note struct {
	crud.BaseModel
	VaultID     int       `gorm:"uniqueIndex:idx_note_vault_path,priority:1" json:"VaultID"`
	RelPath     string    `gorm:"uniqueIndex:idx_note_vault_path,priority:2" json:"RelPath"`
	ParentRel   string    `gorm:"index:idx_note_parent" json:"ParentRel"`
	Name        string    `json:"Name"`
	Title       string    `gorm:"index:idx_note_title" json:"Title"`
	Summary     string    `json:"Summary"`
	SearchText  string    `gorm:"type:text" json:"-"`
	Format      string    `json:"Format"`
	SizeBytes   int       `json:"SizeBytes"`
	ContentHash string    `json:"ContentHash"`
	DiskMTime   time.Time `json:"DiskMTime"`
	DiskMissing bool      `json:"DiskMissing"`
	Starred     bool      `gorm:"index:idx_note_starred" json:"Starred"`
	WordCount   int       `json:"WordCount"`
	TagNames    string    `json:"TagNames"`
	IndexedAt   time.Time `json:"IndexedAt"`
}

func (Note) TableName() string { return "notes" }

// NoteTag 标签。DB 是标签真值来源；front matter 只读不写回。
type NoteTag struct {
	crud.BaseModel
	Name string `gorm:"uniqueIndex" json:"Name"`
}

func (NoteTag) TableName() string { return "note_tags" }

// NoteTagRel 笔记-标签关联（复合主键）。
type NoteTagRel struct {
	NoteID int `gorm:"primaryKey" json:"NoteID"`
	TagID  int `gorm:"primaryKey" json:"TagID"`
}

func (NoteTagRel) TableName() string { return "note_tag_rels" }

// NoteLink [[双链]] 采集；一期只存储，不做图谱（P3 再扩展反向链接面板）。
type NoteLink struct {
	crud.BaseModel
	SourceID  int    `json:"SourceID"`
	TargetRef string `json:"TargetRef"`
	Resolved  bool   `json:"Resolved"`
}

func (NoteLink) TableName() string { return "note_links" }

// resolveSafePath 把 Vault 内的相对路径解析为绝对路径；越界一律拒绝。
// 纯函数，不触碰磁盘。三条硬规则：拒绝 ".."、拒绝绝对路径输入、解析后必须仍在 vault 根之下。
func resolveSafePath(root, rel string) (string, error) {
	if strings.Contains(rel, "\\") {
		rel = strings.ReplaceAll(rel, "\\", "/")
	}
	if strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		return "", ErrInvalidPath
	}
	if strings.ContainsAny(rel, "\x00") {
		return "", ErrInvalidPath
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", ErrPathEscapesVault
	}
	return abs, nil
}

// formatOf 返回文件名的小写扩展名（不含点），无扩展名返回空串。
func formatOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

// wordCount 以空白切分估算词数（中文按字计，足够统计用途）。
func wordCount(s string) int {
	return len(strings.Fields(s))
}
