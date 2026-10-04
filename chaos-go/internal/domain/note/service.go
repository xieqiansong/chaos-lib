package note

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"chaos-go/internal/framework/config"
)

// scanMu 包级互斥：防止并发扫描相互踩踏，第二个请求直接返回 ErrScanInProgress。
var scanMu sync.Mutex

// ListNotes 按筛选条件查询笔记（service 层编排，实际查询在 repository）。
func ListNotes(filter ListFilter) ([]Note, int64, error) {
	return queryNotes(filter)
}

// Tree 由索引构建目录树（含仅作为目录存在的节点）。
func Tree() ([]*TreeNode, error) {
	notes, err := loadAllNotes()
	if err != nil {
		return nil, err
	}
	return buildTree(notes), nil
}

// ScanVault 触发一次 Vault 扫描；模块不可用时返回 ErrVaultUnavailable，并发时返回 ErrScanInProgress。
func ScanVault(full bool) (*ScanResult, error) {
	if !config.GetConfig().Note.Available() {
		return nil, ErrVaultUnavailable
	}
	if !scanMu.TryLock() {
		return nil, ErrScanInProgress
	}
	defer scanMu.Unlock()
	return scanVault(full)
}

// ---------- 写方向用例（P1）----------

// SaveResult 保存成功后的回执。
type SaveResult struct {
	ContentHash string    `json:"contentHash"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ConflictInfo 乐观锁冲突详情，随 409 返回给前端做「覆盖 / 重载」决策。
type ConflictInfo struct {
	BaseHash      string    `json:"baseHash"`      // 客户端提交时的基准 hash
	CurrentHash   string    `json:"currentHash"`   // 磁盘当前 hash
	CurrentSize   int       `json:"currentSize"`   // 磁盘当前字节数
	DiskUpdatedAt time.Time `json:"diskUpdatedAt"` // 磁盘最后修改时间
}

// SaveContent 保存笔记正文（带 baseHash 乐观锁）。
//
// 流程：读取磁盘当前内容 → 算 currentHash；若 baseHash 非空且不等于 currentHash，
// 说明文件已被外部修改（Obsidian / VS Code / git pull 等），返回 ErrConflict 及冲突详情，
// 绝不静默覆盖（这是整个方案唯一「不做就必定丢数据」的环节）。
// 校验通过后原子写回，并按新内容重算索引字段（标题 / 摘要 / 纯文本 / hash / 大小 / 时间）。
func SaveContent(id int, contentStr, baseHash string) (*SaveResult, *ConflictInfo, error) {
	cfg := config.GetConfig().Note
	if !cfg.Available() {
		return nil, nil, ErrVaultUnavailable
	}
	n, err := FindByID(id)
	if err != nil {
		return nil, nil, err
	}
	abs, err := absPath(n.RelPath)
	if err != nil {
		return nil, nil, err
	}
	curBytes, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoteNotFound
		}
		return nil, nil, err
	}
	curHash := sha256hex(curBytes)
	if baseHash != "" && baseHash != curHash {
		var mt time.Time
		if info, e := os.Stat(abs); e == nil {
			mt = info.ModTime()
		}
		return nil, &ConflictInfo{
			BaseHash:      baseHash,
			CurrentHash:   curHash,
			CurrentSize:   len(curBytes),
			DiskUpdatedAt: mt,
		}, ErrConflict
	}
	if len(contentStr) > cfg.LimitBytes() {
		return nil, nil, ErrContentTooLarge
	}
	data := []byte(contentStr)
	if err := WriteContent(n.RelPath, data); err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	updated := parseNoteContent(n.RelPath, n.Name, data, info.ModTime(), int(info.Size()))
	updated.ID = n.ID
	updated.VaultID = n.VaultID
	updated.Starred = n.Starred
	if err := saveNote(&updated); err != nil {
		return nil, nil, err
	}
	return &SaveResult{ContentHash: updated.ContentHash, UpdatedAt: updated.IndexedAt}, nil, nil
}

// CreateNote 在指定目录（parentRel，根级为空串）下新建一个空笔记文件并建索引。
// 缺扩展名时默认补 .md。名称非法或目标已存在分别返回 ErrInvalidPath / ErrAlreadyExists。
func CreateNote(parentRel, name string) (*Note, error) {
	cfg := config.GetConfig().Note
	if !cfg.Available() {
		return nil, ErrVaultUnavailable
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	rel := joinRel(parentRel, name)
	if filepath.Ext(name) == "" {
		rel += ".md"
		name += ".md"
	}
	abs, err := absPath(rel)
	if err != nil {
		return nil, err
	}
	if fileExists(abs) {
		return nil, ErrAlreadyExists
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, []byte(""), 0o644); err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	n := parseNoteContent(rel, name, []byte(""), info.ModTime(), int(info.Size()))
	n.VaultID = DefaultVaultID
	if err := saveNote(&n); err != nil {
		return nil, err
	}
	return &n, nil
}

// RenameNote 重命名笔记文件（仅改文件名，目录不变），并级联更新索引路径。
func RenameNote(id int, newName string) (*Note, error) {
	if err := validateName(newName); err != nil {
		return nil, err
	}
	n, err := FindByID(id)
	if err != nil {
		return nil, err
	}
	newRel := joinRel(n.ParentRel, newName)
	if filepath.Ext(newName) == "" {
		newRel += ".md"
		newName += ".md"
	}
	if err := moveOnDisk(n.RelPath, newRel); err != nil {
		return nil, err
	}
	if err := relocateIndex(n.RelPath, newRel); err != nil {
		return nil, err
	}
	return FindByID(id)
}

// MoveNote 把笔记移动到目标目录（targetDir，根级为空串），并级联更新索引路径。
func MoveNote(id int, targetDir string) (*Note, error) {
	n, err := FindByID(id)
	if err != nil {
		return nil, err
	}
	targetDir = strings.Trim(targetDir, "/")
	newRel := joinRel(targetDir, n.Name)
	if newRel == n.RelPath {
		return n, nil
	}
	if err := moveOnDisk(n.RelPath, newRel); err != nil {
		return nil, err
	}
	if err := relocateIndex(n.RelPath, newRel); err != nil {
		return nil, err
	}
	return FindByID(id)
}

// TrashNote 把笔记移入 .trash 回收站，并硬删索引行（文件保留在回收站，重新扫描可再次索引）。
func TrashNote(id int) error {
	n, err := FindByID(id)
	if err != nil {
		return err
	}
	if _, err := trashOnDisk(n.RelPath); err != nil {
		return err
	}
	return deleteIndex(id)
}

// joinRel 拼接父目录与文件名得到 Vault 内相对路径（根级 parent 为空串）。
func joinRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return strings.TrimRight(parent, "/") + "/" + name
}

// validateName 校验笔记文件名：非空、不含路径分隔符、不含 ".."。
func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidPath
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return ErrInvalidPath
	}
	return nil
}

// ---------- 目录树构建（P0 既有）----------

// buildTree 把扁平的笔记列表聚合成嵌套目录树。
func buildTree(notes []Note) []*TreeNode {
	nodes := make(map[string]*TreeNode, len(notes))
	for i := range notes {
		n := &notes[i]
		nodes[n.RelPath] = &TreeNode{Name: n.Name, RelPath: n.RelPath, IsLeaf: true}
	}
	// 确保目录节点存在（某些父目录本身不是笔记文件）。
	for i := range notes {
		n := &notes[i]
		if n.ParentRel == "" {
			continue
		}
		if _, ok := nodes[n.ParentRel]; !ok {
			nodes[n.ParentRel] = &TreeNode{Name: filepathBase(n.ParentRel), RelPath: n.ParentRel, IsLeaf: false}
		}
	}

	roots := make([]*TreeNode, 0)
	for rel, node := range nodes {
		parent := parentRel(rel)
		if parent == "" {
			roots = append(roots, node)
			continue
		}
		if p, ok := nodes[parent]; ok {
			node.IsLeaf = false
			p.Children = append(p.Children, node)
		} else {
			// 父目录不在索引中（孤立路径），归到根。
			roots = append(roots, node)
		}
	}
	sortNodes(roots)
	return roots
}

func sortNodes(nodes []*TreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		// 目录优先于文件，再按名称。
		if nodes[i].IsLeaf != nodes[j].IsLeaf {
			return !nodes[i].IsLeaf
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, n := range nodes {
		sortNodes(n.Children)
	}
}

func filepathBase(rel string) string {
	if i := lastSlash(rel); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
