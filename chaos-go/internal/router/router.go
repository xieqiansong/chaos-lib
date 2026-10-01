package router

import (
	"chaos-go/internal/apilog"
	"chaos-go/internal/cronjob"
	"chaos-go/internal/datacache"
	"chaos-go/internal/dbmonitor"
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	mqttsync "chaos-go/internal/mqttsync"
	notifysvc "chaos-go/internal/notify"
	"chaos-go/internal/portfwd"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/sdk"
	"chaos-go/internal/standarddata"
	"chaos-go/internal/stunpf"
	stunsync "chaos-go/internal/stunsync"
	"chaos-go/internal/taskplan"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetupRouter(webFS fs.FS) *gin.Engine {
	r := gin.Default()

	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// 接口访问日志：记录每个 /api 请求的元数据与请求/响应体，异步批量落库。
	r.Use(apilog.Middleware())

	api := r.Group("/api")
	{
		api.GET("/browserHistories", proxy.GetBrowserHistories)
		api.POST("/browserHistories", proxy.SaveBrowserHistory)
		api.POST("/browserHistoryVisits", proxy.SaveBrowserHistoryVisits)

		// 常用书签：独立接口（书签 ∪ 历史访问次数，按访问频率排序，分页）
		api.GET("/frequentBookmarks", proxy.GetFrequentBookmarks)
		api.POST("/bookmarks", proxy.SaveBookmarks)

		// SDK 版本与类型管理：资源接口自包含，本行仅做编排调用（路由实现在 internal/sdk）
		sdk.Register(api)

		// 文件连接：资源接口自包含，本行仅做编排调用（路由实现在 internal/filelink）
		filelink.Register(api)

		// 标准参考表：资源接口自包含，本行仅做编排调用（路由实现在 internal/standarddata）
		standarddata.Register(api)

		// 接口访问日志（标准数据范式）：资源接口自包含，本行仅做编排调用（路由实现在 internal/apilog）
		apilog.Register(api)

		// 数据缓存：资源接口自包含，本行仅做编排调用（路由实现在 internal/datacache）
		datacache.Register(api)

		// 快捷编辑：资源接口自包含，本行仅做编排调用（路由实现在 internal/quickedit）
		quickedit.Register(api)

		envVars := api.Group("/envVariables")
		{
			envVars.GET("/", envvar.GetEnvVariables)
			envVars.PATCH("/", envvar.PatchEnvVariables)
			envVars.PUT("/", envvar.PutEnvVariables)
			envVars.POST("/sync", envvar.SyncEnvVariables)
			envVars.GET("/snapshots/:snapshotId", envvar.GetEnvSnapshotDetail)
		}

		// MQTT 多节点消息同步：资源接口自包含，本行仅做编排调用（路由实现在 internal/mqttsync）
		mqttsync.Register(api)

		// 任务计划与待办任务：资源接口自包含，本行仅做编排调用（路由实现在 internal/taskplan）
		taskplan.Register(api)

		// 定时任务（独立模块，与任务计划 / 待办任务无关）：资源接口自包含，
		// 本行仅做编排调用（路由实现在 internal/cronjob）
		cronjob.Register(api)

		// 由原系统内置周期任务改造而来的内部动作接口，供定时任务模块通过 HTTP 触发
		sysJobs := api.Group("/systemJobs")
		{
			sysJobs.POST("/sweep", func(c *gin.Context) {
				taskplan.SweepScheduledTaskPlans()
				renv.Success(c, nil)
			})
			sysJobs.POST("/portForwardSelfHeal", func(c *gin.Context) {
				portfwd.SelfHealForwards()
				renv.Success(c, nil)
			})
			sysJobs.POST("/stunRuleSync", func(c *gin.Context) {
				stunsync.RunSync()
				renv.Success(c, nil)
			})
			sysJobs.POST("/stunPortForwardSync", func(c *gin.Context) {
				stunpf.RunSync()
				renv.Success(c, nil)
			})
		}

		notify := api.Group("/notify")
		{
			notify.POST("/", notifysvc.ShowNotify)
		}

		// 项目管理：资源接口自包含，本行仅做编排调用（路由实现在 internal/project）
		project.Register(api)

		api.GET("/balance/deepseek", proxy.GetDeepSeekBalance)
		api.GET("/weather", proxy.GetWeather)
		api.GET("/hostname", proxy.GetHostname)
		api.GET("/favicon/:host", proxy.GetFavicon)

		// SSH 端口转发：连接与转发规则两个资源接口自包含，本行仅做编排调用（路由实现在 internal/portfwd）
		portfwd.Register(api)
	}

	// 数据库监控（只读自省：表名 / 大小 / 行数 / 索引等统计）；路由由本业务包自包含挂载
	dbmonitor.Register(api)

	r.GET("/", func(c *gin.Context) {
		f, err := webFS.Open("index.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	r.GET("/favicon.svg", func(c *gin.Context) {
		serveUIFile(c, webFS, "favicon.svg", "image/svg+xml")
	})
	r.GET("/favicon.ico", func(c *gin.Context) {
		serveUIFile(c, webFS, "favicon.ico", "image/x-icon")
	})

	r.GET("/assets/*filepath", func(c *gin.Context) {
		filePath := c.Param("filepath")
		if strings.HasPrefix(filePath, "/") {
			filePath = strings.TrimPrefix(filePath, "/")
		}
		f, err := webFS.Open("assets/" + filePath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, getContentType(filePath), content)
	})

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/debug/") {
			return
		}
		f, err := webFS.Open("index.html")
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		content, _ := io.ReadAll(f)
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	return r
}

// serveUIFile 从前端产物根目录读取静态文件并以指定 content-type 返回（favicon 等）。
func serveUIFile(c *gin.Context, webFS fs.FS, name, contentType string) {
	f, err := webFS.Open(name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()
	content, _ := io.ReadAll(f)
	c.Data(http.StatusOK, contentType, content)
}

func getContentType(filename string) string {
	switch filepath.Ext(filename) {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff", ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}
