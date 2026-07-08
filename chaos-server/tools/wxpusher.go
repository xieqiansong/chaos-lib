package tools

import "fmt"

func SendMessage(title, content string) {
	fmt.Printf(" [notify] %s: %s\n", title, content)
	ShowNotify(NotifyConfig{
		Title:       title,
		Text:        content,
		OKLabel:     "确定",
		CancelLabel: "取消",
	})
}
