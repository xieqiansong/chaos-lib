package router

import (
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/middleware"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/routehub"
	"chaos-go/internal/framework/web"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

func SetupRouter(webFS fs.FS) http.Handler {
	r := web.NewRouter()

	r.Use(apilog.Middleware())

	api := r.Group("/api")

	v1 := api.Group("/v1")
	v1.Use(envelope.Middleware())
	v1.Use(middleware.AuthMiddleware())
	if mounted := routehub.MountAllV1(v1); len(mounted) > 0 {
		slog.Info("v1 路由模块挂载完成", "modules", strings.Join(mounted, ", "))
	}

	plugin.MountAll(v1)

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
