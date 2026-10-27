package proxy

import (
	"net/http"

	"chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

func metaToQuery(c *web.Context) {
	envelope.MetaToQuery(c)
}

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/proxy")

	g.GET("/favicon/:host", func(c *web.Context) { GetFavicon(c, c.Param("host")) })

	bh := g.Group("/browserHistories")
	bh.POST("/list", func(c *web.Context) { metaToQuery(c); GetBrowserHistories(c) })
	bh.POST("/save", SaveBrowserHistory)
	bh.POST("/saveVisits", SaveBrowserHistoryVisits)

	bm := g.Group("/bookmarks")
	bm.POST("/getTree", GetBookmarkTree)
	bm.POST("/frequent", func(c *web.Context) { metaToQuery(c); GetFrequentBookmarks(c) })
	bm.POST("/save", SaveBookmarks)

	g.POST("/balance/deepseek", GetDeepSeekBalance)
	g.POST("/weather", func(c *web.Context) { metaToQuery(c); GetWeather(c) })
	g.POST("/hostname", GetHostname)
	g.POST("/favicon", faviconV1)
}

func faviconV1(c *web.Context) {
	var req struct {
		Host string `json:"host"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.Host == "" {
		renv.Error(c, http.StatusBadRequest, "host 不能为空")
		return
	}
	GetFavicon(c, req.Host)
}
