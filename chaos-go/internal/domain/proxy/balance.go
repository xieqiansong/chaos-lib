package proxy

import (
	"encoding/json"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/config"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const deepseekBalanceURL = "https://api.deepseek.com/user/balance"

func GetDeepSeekBalance(c *gin.Context) {
	apiKey := config.GetConfig().DeepSeek.APIKey
	if apiKey == "" {
		renv.Error(c, http.StatusServiceUnavailable, "未配置 DeepSeek API Key，请在服务端 configs/config.yaml 中设置")
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
	// 接口重构：原样透传改为统一信封，便于前端走 action()（POST + Action）消费。
	// DeepSeek 余额响应为 JSON 对象，包进 data；非 JSON 则作为字符串透传。
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		renv.Success(c, string(body))
		return
	}
	renv.Success(c, raw)
}
