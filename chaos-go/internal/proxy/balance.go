package proxy

import (
	renv "chaos-go/internal/resp"
	"chaos-go/internal/config"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const deepseekBalanceURL = "https://api.deepseek.com/user/balance"

func GetDeepSeekBalance(c *gin.Context) {
	apiKey := config.GetConfig().DeepSeekAPIKey
	if apiKey == "" {
		renv.Error(c, http.StatusServiceUnavailable, "未配置 DEEPSEEK_API_KEY，请在服务端 .env 中设置")
		return
	}
	req, err := http.NewRequest(http.MethodGet, deepseekBalanceURL, nil)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "创建请求失败: " + err.Error())
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		renv.Error(c, http.StatusBadGateway, "请求 DeepSeek 失败: " + err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取响应失败: " + err.Error())
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("DeepSeek 余额接口返回错误", "statusCode", resp.StatusCode, "body", string(body))
		renv.Error(c, resp.StatusCode, "DeepSeek 返回错误")
		return
	}
	c.Data(resp.StatusCode, "application/json", body)
}
