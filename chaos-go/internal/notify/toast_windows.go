//go:build windows

package notify

import (
	"log/slog"
	"runtime"
)

// ShowWindowsNotify 通过 Windows Toast 显示通知
func ShowWindowsNotify(title, content string) {
	slog.Info("Windows通知", "title", title, "content", content, "os", runtime.GOOS)
	// TODO: 实现实际的 Windows Toast 通知
}
