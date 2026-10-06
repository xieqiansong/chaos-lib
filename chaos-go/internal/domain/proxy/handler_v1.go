package proxy

import (
	"fmt"
	"net/http"
	"strconv"

	"chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的 v1 路由挂载函数登记到 routehub（与存量 /api 并存，双轨迁移）。
func init() {
	routehub.RegisterV1("proxy", RegisterV1)
}

// metaToQuery 把信封 meta 转为查询参数注入请求，使既有依赖 query 的 handler（分页/搜索）在 v1 下复用。
func metaToQuery(c *gin.Context) {
	var m map[string]any
	envelope.GetMeta(c, &m)
	q := c.Request.URL.Query()
	for k, v := range m {
		key := k
		if key == "pageSize" {
			key = "page_size"
		}
		switch val := v.(type) {
		case string:
			if val != "" {
				q.Set(key, val)
			}
		case float64:
			q.Set(key, strconv.Itoa(int(val)))
		case bool:
			q.Set(key, strconv.FormatBool(val))
		case nil:
			// 跳过空值
		default:
			q.Set(key, fmt.Sprintf("%v", val))
		}
	}
	c.Request.URL.RawQuery = q.Encode()
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
func RegisterV1(rg *gin.RouterGroup) {
	g := rg.Group("/proxy")

	// 浏览器历史 / 书签：列表复用既有 handler（meta→query），保存类已支持信封 data。
	bh := g.Group("/browserHistories")
	bh.POST("/list", func(c *gin.Context) { metaToQuery(c); GetBrowserHistories(c) })
	bh.POST("/save", SaveBrowserHistory)
	bh.POST("/saveVisits", SaveBrowserHistoryVisits)

	bm := g.Group("/bookmarks")
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
