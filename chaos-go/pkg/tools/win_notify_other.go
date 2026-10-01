//go:build !windows

package tools

import "log/slog"

// NotifyConfig 通知窗口配置（非 Windows 平台占位定义，保持与 Windows 版一致以便复用）
type NotifyConfig struct {
	Title       string
	Text        string
	Width       int32
	Height      int32
	OKLabel     string
	CancelLabel string
}

// NotifyResult 用户操作结果（非 Windows 平台占位定义）
type NotifyResult int

const (
	NotifyOK NotifyResult = iota + 1
	NotifyCancel
	NotifyClose
)

// ShowNotify 非 Windows 平台无原生弹窗，记录日志后直接返回 NotifyClose。
func ShowNotify(config NotifyConfig) NotifyResult {
	slog.Info("通知（非 Windows 平台暂不支持原生弹窗）", "title", config.Title, "text", config.Text)
	return NotifyClose
}
