package deepseek

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"chaos-go/config"
)

const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
	chatPath       = "/chat/completions"

	// 单次请求上限，避免长文拖累延迟与费用
	requestTimeout = 60 * time.Second
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 DeepSeek 聊天接口，返回模型回复文本。
// system 为固定的角色/评分标准提示（每次调用相同，省 token），user 为本次具体输入。
// 密钥取自服务端配置 DeepSeek.APIKey，绝不暴露到前端。
func Chat(system, user string) (string, error) {
	apiKey := config.GetConfig().DeepSeek.APIKey
	if apiKey == "" {
		return "", fmt.Errorf("未配置 DEEPSEEK_API_KEY，请在服务端 .env 中设置后使用 AI 评分")
	}

	reqBody := chatRequest{
		Model:       defaultModel,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	reqBody.ResponseFormat.Type = "json_object"

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := defaultBaseURL + chatPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 DeepSeek 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("DeepSeek chat 返回错误", "status", resp.StatusCode, "body", string(body))
		return "", fmt.Errorf("DeepSeek 返回错误(%d): %s", resp.StatusCode, string(body))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 响应失败: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek 未返回内容")
	}
	return cr.Choices[0].Message.Content, nil
}
