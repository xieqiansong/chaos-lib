package proxy

import (
	"encoding/json"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/config"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"chaos-go/internal/framework/web"
)

const baiduWeatherURL = "https://api.map.baidu.com/weather/v1/"

func GetWeather(c *web.Context) {
	ak := config.GetConfig().Baidu.AK
	if ak == "" {
		renv.Error(c, http.StatusServiceUnavailable, "未配置百度地图 AK，请在服务端 configs/config.yaml 中设置")
		return
	}
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "lat 参数无效")
		return
	}
	lon, err := strconv.ParseFloat(c.Query("lon"), 64)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "lon 参数无效")
		return
	}
	location := strconv.FormatFloat(lon, 'f', -1, 64) + "," + strconv.FormatFloat(lat, 'f', -1, 64)
	reqURL := baiduWeatherURL + "?location=" + url.QueryEscape(location) + "&data_type=now&ak=" + url.QueryEscape(ak)
	resp, err := http.Get(reqURL)
	if err != nil {
		renv.Error(c, http.StatusBadGateway, "请求百度天气失败: " + err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取响应失败: " + err.Error())
		return
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("百度天气接口返回错误", "statusCode", resp.StatusCode, "body", string(body))
	}
	// 接口重构：原始百度 JSON 透传改为统一信封，便于前端走 action()（POST + Action）消费；
	// 数据形状不变（data.status / data.result 仍可读）。
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		renv.Success(c, string(body))
		return
	}
	renv.Success(c, raw)
}
