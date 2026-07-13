package tools

import "log/slog"

func SendMessage(title, content string) {
	slog.Info("发送通知", "title", title, "content", content)
	ShowNotify(NotifyConfig{
		Title:       title,
		Text:        content,
		OKLabel:     "确定",
		CancelLabel: "取消",
	})
}
