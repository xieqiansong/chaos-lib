//go:build !windows

package tools

import (
	"os"
	"time"
)

// DirCreatedAt 非 Windows 平台无法可靠获取目录创建时间，返回零值（调用方回退为当前时间）。
func DirCreatedAt(path string) time.Time {
	if _, err := os.Stat(path); err != nil {
		return time.Time{}
	}
	return time.Time{}
}
