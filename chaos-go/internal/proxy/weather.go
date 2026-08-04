package proxy

import (
	"chaos-go/config"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

const baiduWeatherURL = "https://api.map.baidu.com/weather/v1/"

func GetWeather(c *gin.Context) {
	ak := config.GetConfig().Baidu.AK
	if ak == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "未配置 BAIDU_AK，请在服务端 .env 中设置"})
		return
	}
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat 参数无效"})
		return
	}
	lon, err := strconv.ParseFloat(c.Query("lon"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lon 参数无效"})
		return
	}
	location := strconv.FormatFloat(lon, 'f', -1, 64) + "," + strconv.FormatFloat(lat, 'f', -1, 64)
	reqURL := baiduWeatherURL + "?location=" + url.QueryEscape(location) + "&data_type=now&ak=" + url.QueryEscape(ak)
	resp, err := http.Get(reqURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求百度天气失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应失败: " + err.Error()})
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("百度天气接口返回错误", "statusCode", resp.StatusCode, "body", string(body))
	}
	c.Data(resp.StatusCode, "application/json; charset=utf-8", body)
}
