package apilog

import (
	"chaos-go/internal/framework/config"
	"log/slog"
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

// logCh 异步落库缓冲通道（采集端 middleware → 持久化端 flushLoop）。
var logCh = make(chan *ApiLog, 2000)

// Start 启动后台消费者，由启动流程显式调用（不在 init 里起 goroutine）。
func Start() {
	go flushLoop()
}

// flushLoop 后台消费者：累计 100 条或每 2s 批量落库一次。
func flushLoop() {
	const batchSize = 100
	batch := make([]*ApiLog, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := config.GetDB().Create(batch).Error; err != nil {
			slog.Error("api 日志批量落库失败", "err", err)
		}
		batch = batch[:0]
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case rec := <-logCh:
			batch = append(batch, rec)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
