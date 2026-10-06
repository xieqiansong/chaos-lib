package router

import (
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/middleware"
	"chaos-go/internal/framework/routehub"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetupRouter 装配 HTTP 路由：全局中间件在此编排，业务模块路由由 internal/routehub
// 统一挂载（各模块在自己的包内实现 Register 并登记），静态资源与 SPA 回退在此兜底。
func SetupRouter(webFS fs.FS) *gin.Engine {
	r := gin.Default()

	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// 接口访问日志：记录每个 /api 请求的元数据与请求/响应体，异步批量落库。
	r.Use(apilog.Middleware())

	api := r.Group("/api")

	// 业务模块路由：各模块在自身包内实现 Register(rg *gin.RouterGroup)，并在 init 中
	// 登记到 internal/framework/routehub；各业务包由 internal/app 装配根统一 blank-import，
	// 于是新增模块只需建包并登记，本文件不必再改动。
	// 将来若出现跨模块的路由前缀冲突（如 /xx/:id 与 /xx/new 重叠），
	// 可把冲突模块退回此处显式调用其 Register，以确定挂载顺序。
	if mounted := routehub.MountAll(api); len(mounted) > 0 {
		slog.Info("路由模块挂载完成", "modules", strings.Join(mounted, ", "))
	}

	// v1 接口组：统一「POST + Action」模式（详见仓库根《接口规范.md》）。
	// 信封中间件只作用于本组，存量 /api 路由不受影响；迁移中的模块经 routehub.RegisterV1
	// 登记后挂到此处，未迁移模块仍走 /api，实现双轨兼容、可随时回退。
	v1 := api.Group("v1")
	v1.Use(envelope.Middleware())
	v1.Use(middleware.AuthMiddleware())
	if mounted := routehub.MountAllV1(v1); len(mounted) > 0 {
		slog.Info("v1 路由模块挂载完成", "modules", strings.Join(mounted, ", "))
	}

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
