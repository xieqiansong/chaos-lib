package filelink

import (
	"os"
	"path/filepath"
	"strings"
)

// normalizeLinkPath 去掉 os.Readlink 返回的卷命名空间前缀（\??\ 或 \\?\），
// 还原为普通盘符路径，便于与源路径比较。
func normalizeLinkPath(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + p[len(`\\?\UNC\`):]
	}
	if strings.HasPrefix(p, `\??\UNC\`) {
		return `\\` + p[len(`\??\UNC\`):]
	}
	if strings.HasPrefix(p, `\??\`) || strings.HasPrefix(p, `\\?\`) {
		return p[4:]
	}
	return p
}

// checkLinkStatus 按文件系统真实状态推导连接状态（不落库、每次读时重算）：
// normal 正常 / missing 目标缺失 / none 未启用 / invalid 无效 / conflict 冲突。
func checkLinkStatus(sourcePath, targetPath string, enabled bool) string {
	_, err := os.Lstat(targetPath)
	if err != nil {
		if enabled {
			return "missing"
		}
		return "none"
	}
	if !enabled {
		return "invalid"
	}
	actualTarget, err := os.Readlink(targetPath)
	if err == nil {
		absActual, _ := filepath.Abs(normalizeLinkPath(actualTarget))
		absSource, _ := filepath.Abs(sourcePath)
		if strings.EqualFold(absActual, absSource) {
			return "normal"
		}
	}
	return "conflict"
}
