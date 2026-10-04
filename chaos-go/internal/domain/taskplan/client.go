package taskplan

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// 外部 HTTP 客户端（基础设施层），只被 service 调用。

// rawFetchTimeout 拉取原文的单次超时。
const rawFetchTimeout = 15 * time.Second

// fetchRawContent 拉取给定 URL 的文本内容（用于代理 chaos-knots 的 raw 接口）。
func fetchRawContent(rawURL string) (string, error) {
	client := &http.Client{Timeout: rawFetchTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
