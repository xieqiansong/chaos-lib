package note

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaos-go/internal/config"
)

// scanVault 遍历 Vault 根目录，增量更新索引；由 service.ScanVault 在加锁后调用。
// 快筛优先：先比 size+mtime，仅变化才读内容算 hash；hash 未变只刷新磁盘元数据。
// 缺失不断言：磁盘消失只置 disk_missing，避免外接盘未挂载时整库误判。
func scanVault(full bool) (*ScanResult, error) {
	cfg := config.GetConfig().Note
	root := cfg.Root()
	if root == "" {
		return nil, ErrVaultUnavailable
	}
	exts := make(map[string]bool)
	for _, e := range cfg.Exts() {
		exts[strings.ToLower(e)] = true
	}
	ignores := cfg.Ignores()
	limit := cfg.LimitBytes()

	existing, err := loadAllNotes()
	if err != nil {
		return nil, err
	}
	index := make(map[string]*Note, len(existing))
	for i := range existing {
		index[existing[i].RelPath] = &existing[i]
	}
	seen := make(map[string]bool)
	res := &ScanResult{}

	start := time.Now()

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // 不可读条目跳过（权限 / 外接盘未挂载）
		}
		if d.IsDir() {
			if path != root && inIgnores(relOf(root, path), ignores) {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), "."))
		if !exts[ext] {
			return nil
		}
		rel := relOf(root, path)
		if rel == "" || inIgnores(rel, ignores) {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		size := int(info.Size())
		seen[rel] = true
		if size > limit {
			res.Skipped++
			return nil
		}
		mtime := info.ModTime()

		if old, ok := index[rel]; ok {
			// 快筛：大小一致且磁盘未更新 → 跳过。
			if !full && old.SizeBytes == size && !mtime.After(old.DiskMTime) {
				return nil
			}
			content, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			if old.ContentHash == sha256hex(content) {
				old.SizeBytes = size
				old.DiskMTime = mtime
				old.DiskMissing = false
				_ = saveNote(old)
				return nil
			}
			parsed := parseNoteContent(rel, d.Name(), content, mtime, size)
			parsed.ID = old.ID
			parsed.VaultID = old.VaultID
			parsed.Starred = old.Starred
			if saveNote(&parsed) == nil {
				res.Updated++
			}
			return nil
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		parsed := parseNoteContent(rel, d.Name(), content, mtime, size)
		parsed.VaultID = DefaultVaultID
		if saveNote(&parsed) == nil {
			res.Added++
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	// 扫描后比对：在盘缺失项置 true，重新出现且曾缺失的项复位为 false。
	var missing, reset []string
	for rel, n := range index {
		if seen[rel] {
			if n.DiskMissing {
				reset = append(reset, rel)
			}
		} else if !n.DiskMissing {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		if err := markMissing(missing, true); err != nil {
			return nil, err
		}
		res.Missing = len(missing)
	}
	if len(reset) > 0 {
		if err := markMissing(reset, false); err != nil {
			return nil, err
		}
	}

	res.Scanned = len(seen)
	res.ElapsedMs = time.Since(start).Milliseconds()
	return res, nil
}

// parseNoteContent 由文件内容派生一条索引记录（纯派生，可随重扫重建）。
// 标题 / 摘要 / 纯文本走 parser 包已实现的 ExtractTitle / Summarize / PlainText。
func parseNoteContent(rel, name string, content []byte, mtime time.Time, size int) Note {
	fallback := strings.TrimSuffix(name, filepath.Ext(name))
	plain := PlainText(content)
	return Note{
		VaultID:     DefaultVaultID,
		RelPath:     rel,
		ParentRel:   parentRel(rel),
		Name:        name,
		Title:       ExtractTitle(content, fallback),
		Summary:     Summarize(content, 200),
		SearchText:  plain,
		Format:      formatOf(name),
		SizeBytes:   size,
		ContentHash: sha256hex(content),
		DiskMTime:   mtime,
		WordCount:   wordCount(plain),
		IndexedAt:   time.Now(),
	}
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// relOf 计算 path 相对 root 的 '/'-分隔路径；无法计算返回空串。
func relOf(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// parentRel 返回 rel 的父目录相对路径，根级返回空串。
func parentRel(rel string) string {
	if rel == "" {
		return ""
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

// inIgnores 判断 rel 的任意路径段是否命中忽略清单（避免把 .git / node_modules 扫进索引）。
func inIgnores(rel string, ignores []string) bool {
	if len(ignores) == 0 {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		for _, ig := range ignores {
			if seg == ig {
				return true
			}
		}
	}
	return false
}
