package proxy

import (
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("proxy", Register)
}

// Register 挂载代理类接口：浏览器历史、书签，以及天气 / 主机名 / 站点图标 / 余额查询等转发型接口。
func Register(rg *gin.RouterGroup) {
	rg.GET("/browserHistories", GetBrowserHistories)
	rg.POST("/browserHistories", SaveBrowserHistory)
	rg.POST("/browserHistoryVisits", SaveBrowserHistoryVisits)

	// 常用书签：独立接口（书签 ∪ 历史访问次数，按访问频率排序，分页）
	rg.GET("/frequentBookmarks", GetFrequentBookmarks)
	rg.POST("/bookmarks", SaveBookmarks)

	rg.GET("/balance/deepseek", GetDeepSeekBalance)
	rg.GET("/weather", GetWeather)
	rg.GET("/hostname", GetHostname)
	rg.GET("/favicon/:host", GetFavicon)
}
