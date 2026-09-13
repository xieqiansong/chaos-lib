//go:build !windows

package notify

import "log/slog"

// ShowWindowsNotify 非 Windows 平台暂不支持原生通知，仅记录日志。
func ShowWindowsNotify(title, content string) {
	slog.Info("通知（非 Windows 平台暂不支持原生通知）", "title", title, "content", content)
}
