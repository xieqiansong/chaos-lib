package note

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaos-go/internal/framework/config"
)

// trashDir Vault 内的回收站目录名；config 默认忽略清单已含此目录，扫描不会重新索引。
const trashDir = ".trash"

// ReadContent 按索引项读取磁盘原文（只读）。
// 路径经 resolveSafePath 严格校验：拒绝 ".."、拒绝绝对路径输入、解析后必须仍在 Vault 根下。
func ReadContent(n *Note) (string, error) {
	root := config.GetConfig().Note.Root()
	if root == "" {
		return "", ErrVaultUnavailable
	}
	abs, err := resolveSafePath(root, n.RelPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoteNotFound
		}
		return "", err
	}
	return string(data), nil
}

// absPath 把 Vault 内相对路径解析为受校验的绝对路径；模块不可用或越界均报错。
func absPath(rel string) (string, error) {
	root := config.GetConfig().Note.Root()
	if root == "" {
		return "", ErrVaultUnavailable
	}
	return resolveSafePath(root, rel)
}

// fileExists 判断给定绝对路径是否存在且为普通文件（非目录）。
func fileExists(abs string) bool {
	info, err := os.Stat(abs)
	return err == nil && !info.IsDir()
}

// WriteContent 原子写回：先写临时文件再 rename，避免半截文件被外部工具读到。
// 路径经 resolveSafePath 严格校验；父目录不存在时自动创建。
func WriteContent(rel string, content []byte) error {
	abs, err := absPath(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp-%d", abs, time.Now().UnixNano())
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, abs)
}

// moveOnDisk 在 Vault 内移动/重命名文件（磁盘层，不含索引更新）。
// 目标已存在返回 ErrAlreadyExists；源不存在返回 ErrNoteNotFound。
func moveOnDisk(oldRel, newRel string) error {
	oldAbs, err := absPath(oldRel)
	if err != nil {
		return err
	}
	newAbs, err := absPath(newRel)
	if err != nil {
		return err
	}
	if !fileExists(oldAbs) {
		return ErrNoteNotFound
	}
	if fileExists(newAbs) {
		return ErrAlreadyExists
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}

// trashOnDisk 把文件移入 Vault 内 .trash 目录（同名冲突时追加序号去重），
// 返回实际使用的回收站相对路径。
func trashOnDisk(rel string) (string, error) {
	root := config.GetConfig().Note.Root()
	if root == "" {
		return "", ErrVaultUnavailable
	}
	abs, err := resolveSafePath(root, rel)
	if err != nil {
		return "", err
	}
	if !fileExists(abs) {
		return "", ErrNoteNotFound
	}
	trashRel := filepath.ToSlash(filepath.Join(trashDir, rel))
	trashAbs, err := resolveSafePath(root, trashRel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(trashAbs), 0o755); err != nil {
		return "", err
	}
	final := trashAbs
	ext := filepath.Ext(trashAbs)
	base := strings.TrimSuffix(trashAbs, ext)
	for fileExists(final) {
		// 仅在确需去重时追加序号，避免覆盖已有回收文件。
		base = fmt.Sprintf("%s_%d", base, time.Now().UnixNano())
		final = base + ext
	}
	if err := os.Rename(abs, final); err != nil {
		return "", err
	}
	used, err := filepath.Rel(root, final)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(used), nil
}
