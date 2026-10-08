package router

import (
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/middleware"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/framework/web"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

// SetupRouter 装配 HTTP 路由：全局中间件在此编排，业务模块的 v1 路由由 internal/routehub
// 统一挂载（各模块实现 RegisterV1 并登记），静态资源与 SPA 回退在此兜底。
// 返回 http.Handler，可直接交给 http.ListenAndServe 或 HTTP/3 服务。
func SetupRouter(webFS fs.FS) http.Handler {
	r := web.NewRouter()

	// 响应 gzip 压缩（替代 gin-contrib/gzip）。
	// 接口访问日志：记录每个 /api 请求的元数据与请求/响应体，异步批量落库。
	r.Use(apilog.Middleware())

	api := r.Group("/api")

	// v1 接口组：统一「POST + Action」模式（详见仓库根《接口规范.md》）。
	// 所有业务接口均已迁到本组，存量 RESTful /api 路由与其兼容层已下线移除。
	v1 := api.Group("/v1")
	v1.Use(envelope.Middleware())
	v1.Use(middleware.AuthMiddleware())
	if mounted := routehub.MountAllV1(v1); len(mounted) > 0 {
		slog.Info("v1 路由模块挂载完成", "modules", strings.Join(mounted, ", "))
	}

	// 根路径 "/" 由下方 NoRoute 的 "/{path...}" 兜底（同样返回 index.html），
	// 这里不再单独注册 GET /，否则会与通配模式在 Go ServeMux 中判定为冲突。

	r.GET("/favicon.svg", func(c *web.Context) {
		serveUIFile(c, webFS, "favicon.svg", "image/svg+xml")
	})
	r.GET("/favicon.ico", func(c *web.Context) {
		serveUIFile(c, webFS, "favicon.ico", "image/x-icon")
	})

	r.GET("/assets/*filepath", func(c *web.Context) {
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

	r.NoRoute(func(c *web.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/debug/") {
			c.Status(http.StatusNotFound)
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
func serveUIFile(c *web.Context, webFS fs.FS, name, contentType string) {
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
