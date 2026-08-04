package proxy

import (
	"chaos-go/config"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const deepseekBalanceURL = "https://api.deepseek.com/user/balance"

func GetDeepSeekBalance(c *gin.Context) {
	apiKey := config.GetConfig().DeepSeek.APIKey
	if apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "未配置 DEEPSEEK_API_KEY，请在服务端 .env 中设置"})
		return
	}
	req, err := http.NewRequest(http.MethodGet, deepseekBalanceURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建请求失败: " + err.Error()})
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求 DeepSeek 失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应失败: " + err.Error()})
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("DeepSeek 余额接口返回错误", "statusCode", resp.StatusCode, "body", string(body))
		c.JSON(resp.StatusCode, gin.H{"error": "DeepSeek 返回错误", "detail": string(body)})
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}
