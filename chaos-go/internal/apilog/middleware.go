package apilog

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware 记录每个 /api 请求的元数据，投递到异步落库通道（channel 满则丢弃，避免阻塞请求链路）。
// 非 /api 路径（静态资源、SPA 回退等）直接放行，不记录。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		rec := &ApiLog{
			Path:       c.Request.URL.Path,
			Method:     c.Request.Method,
			StatusCode: c.Writer.Status(),
			LatencyMs:  time.Since(start).Milliseconds(),
			ClientIP:   c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			BodySize:   c.Writer.Size(),
		}
		if errs := c.Errors; len(errs) > 0 {
			rec.ErrorMsg = errs[0].Error()
		}
		select {
		case logCh <- rec:
		default:
			// 通道满则丢弃，避免阻塞请求链路。
		}
	}
}
