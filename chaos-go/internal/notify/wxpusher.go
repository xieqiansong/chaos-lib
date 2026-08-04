package notify

import "log/slog"

// SendWxPusher 通过 WxPusher 发送消息
func SendWxPusher(token, title, content string) {
	slog.Info("WxPusher", "title", title, "content", content)
	// TODO: 实现实际的 WxPusher API 调用
}
