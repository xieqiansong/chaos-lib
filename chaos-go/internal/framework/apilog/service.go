package apilog

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/web"
	"log/slog"
	"strings"
	"time"
)

func Middleware() web.HandlerFunc {
	return func(c *web.Context) {
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
		if errs := c.Errors(); len(errs) > 0 {
			rec.ErrorMsg = errs[0].Error()
		}
		select {
		case logCh <- rec:
		default:

		}
	}
}

var logCh = make(chan *ApiLog, 2000)

func Start() {
	go flushLoop()
}

func flushLoop() {
	const batchSize = 100
	batch := make([]*ApiLog, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		db := config.GetDB()
		if db == nil {

			batch = batch[:0]
			return
		}
		if err := db.Create(batch).Error; err != nil {
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
