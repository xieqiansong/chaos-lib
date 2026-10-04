package note

import "time"

// SaveContentRequest PUT /:id/content 请求体：正文 + 乐观锁基准 hash。
type SaveContentRequest struct {
	Content string `json:"content"`
	BaseHash string `json:"baseHash"`
}

// CreateRequest POST /create 请求体：新建笔记。
type CreateRequest struct {
	ParentRel string `json:"parentRel"`
	Name      string `json:"name"`
}

// RenameRequest POST /:id/rename 请求体。
type RenameRequest struct {
	Name string `json:"name"`
}

// MoveRequest POST /:id/move 请求体：目标目录（根级为空串）。
type MoveRequest struct {
	TargetDir string `json:"targetDir"`
}

// NoteResponse 笔记索引项的对外形态（不含 SearchText 等派生内部字段）。
type NoteResponse struct {
	ID          int       `json:"ID"`
	VaultID     int       `json:"VaultID"`
	RelPath     string    `json:"RelPath"`
	ParentRel   string    `json:"ParentRel"`
	Name        string    `json:"Name"`
	Title       string    `json:"Title"`
	Summary     string    `json:"Summary"`
	Format      string    `json:"Format"`
	SizeBytes   int       `json:"SizeBytes"`
	DiskMTime   time.Time `json:"DiskMTime"`
	DiskMissing bool      `json:"DiskMissing"`
	Starred     bool      `json:"Starred"`
	WordCount   int       `json:"WordCount"`
	TagNames    string    `json:"TagNames"`
	IndexedAt   time.Time `json:"IndexedAt"`
	CreatedAt   time.Time `json:"CreatedAt"`
	UpdatedAt   time.Time `json:"UpdatedAt"`
}

// TreeNode 目录树节点（前端 el-tree 使用）。
type TreeNode struct {
	Name     string      `json:"name"`
	RelPath  string      `json:"relPath"`
	IsLeaf   bool        `json:"isLeaf"`
	Children []*TreeNode `json:"children,omitempty"`
}

// ScanResult 扫描统计结果。
type ScanResult struct {
	Scanned   int   `json:"scanned"`
	Added     int   `json:"added"`
	Updated   int   `json:"updated"`
	Missing   int   `json:"missing"`
	Skipped   int   `json:"skipped"`
	ElapsedMs int64 `json:"elapsedMs"`
}

// ToNoteResponses 读方向批量回调：把 []*Note 转换为 []NoteResponse（供 crud 基线复用）。
func ToNoteResponses(rows []*Note) any {
	out := make([]NoteResponse, 0, len(rows))
	for _, n := range rows {
		out = append(out, toNoteResponse(*n))
	}
	return out
}

func toNoteResponse(n Note) NoteResponse {
	return NoteResponse{
		ID:          n.ID,
		VaultID:     n.VaultID,
		RelPath:     n.RelPath,
		ParentRel:   n.ParentRel,
		Name:        n.Name,
		Title:       n.Title,
		Summary:     n.Summary,
		Format:      n.Format,
		SizeBytes:   n.SizeBytes,
		DiskMTime:   n.DiskMTime,
		DiskMissing: n.DiskMissing,
		Starred:     n.Starred,
		WordCount:   n.WordCount,
		TagNames:    n.TagNames,
		IndexedAt:   n.IndexedAt,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}
}
