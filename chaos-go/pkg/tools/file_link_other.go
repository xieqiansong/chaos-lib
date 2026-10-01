//go:build !windows

package tools

import "os"

// CreateDirectoryLink 在非 Windows 平台上创建目录符号链接。
func CreateDirectoryLink(junctionPath, targetDir string) error {
	return os.Symlink(targetDir, junctionPath)
}
