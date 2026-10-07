package proxy

import (
	"net/http"

	"chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的 v1 路由挂载函数登记到 routehub（与存量 /api 并存，双轨迁移）。
func init() {
	routehub.RegisterV1("proxy", RegisterV1)
}

// metaToQuery 把信封 meta 桥接为查询参数，使既有依赖 query 的 handler（分页/搜索）在 v1 下复用。
// 实现已上收到 framework/envelope.MetaToQuery，本处仅为路由注册处的可读性保留薄封装。
func metaToQuery(c *gin.Context) {
	envelope.MetaToQuery(c)
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
func RegisterV1(rg *gin.RouterGroup) {
	g := rg.Group("/proxy")

	// favicon 为图片流，保留 GET 端点供前端 <img> 直连（与「POST + Action」规范互补）。
	g.GET("/favicon/:host", GetFavicon)

	// 浏览器历史 / 书签：列表复用既有 handler（meta→query），保存类已支持信封 data。
	bh := g.Group("/browserHistories")
	bh.POST("/list", func(c *gin.Context) { metaToQuery(c); GetBrowserHistories(c) })
	bh.POST("/save", SaveBrowserHistory)
	bh.POST("/saveVisits", SaveBrowserHistoryVisits)

	bm := g.Group("/bookmarks")
	bm.POST("/getTree", GetBookmarkTree)
	bm.POST("/frequent", func(c *gin.Context) { metaToQuery(c); GetFrequentBookmarks(c) })
	bm.POST("/save", SaveBookmarks)

	// 代理查询类：多数无参数，weather 的 q 经 meta→query 传入。
	g.POST("/balance/deepseek", GetDeepSeekBalance)
	g.POST("/weather", func(c *gin.Context) { metaToQuery(c); GetWeather(c) })
	g.POST("/hostname", GetHostname)
	g.POST("/favicon", faviconV1)
}

// faviconV1 是 GetFavicon 的「POST + Action」版：host 取自信封 data，注入路径参数后复用原 handler。
func faviconV1(c *gin.Context) {
	var req struct {
		Host string `json:"host"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.Host == "" {
		renv.Error(c, http.StatusBadRequest, "host 不能为空")
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "host", Value: req.Host})
	GetFavicon(c)
}
