// Package apilog 接口访问日志：记录每个 /api 请求的元数据，
// 作为「标准数据」落库（嵌入 crud.BaseModel，复用标准 CRUD 基线，前端可分页检索）。
//
// 设计要点：
//   - 通过自定义 gin 中间件采集请求时间、相应用时、状态码等元数据（不记录请求/响应体）；
//   - 落库走「缓冲通道 + 后台批处理」异步写入，避免每个请求都同步写库造成链路抖动；
//   - 通道满时丢弃当前日志（不阻塞请求），后台每 2s 或累计 100 条触发一次批量落库。
package apilog

import (
	"log/slog"
	"strings"
	"time"

	"chaos-go/internal/config"
	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
)

// ApiLog 单条接口访问日志（标准数据范式：嵌入 BaseModel 获得 ID/创建/更新/软删）。
type ApiLog struct {
	crud.BaseModel
	Path         string `json:"Path"`
	Method       string `json:"Method"`
	StatusCode   int    `json:"StatusCode"`
	LatencyMs    int64  `json:"LatencyMs"` // 相应用时（毫秒）
	ClientIP     string `json:"ClientIP"`
	UserAgent    string `json:"UserAgent"`
	BodySize     int    `json:"BodySize"`
	ErrorMsg     string `json:"ErrorMsg"`
}

// TableName 显式指定表名（与前端资源名 apiLog 对应）。
func (ApiLog) TableName() string { return "api_logs" }

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 纯 CRUD 交给通用 crud，前端按路径/方法/错误信息搜索、按耗时/状态排序。
func Register(rg *gin.RouterGroup) {
	crud.Register[ApiLog](rg, "apiLog", crud.Opts[ApiLog]{
		Searchable: []string{"path", "method", "error_msg"},
		Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
	})
}

// logCh 异步落库缓冲通道。
var logCh = make(chan *ApiLog, 2000)

func init() {
	go flushLoop()
}

// Middleware 记录每个 /api 请求的元数据，异步批量落库。
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
			Path:         c.Request.URL.Path,
			Method:       c.Request.Method,
			StatusCode:   c.Writer.Status(),
			LatencyMs:    time.Since(start).Milliseconds(),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			BodySize:     c.Writer.Size(),
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

// flushLoop 后台消费者：累计 100 条或每 2s 批量落库一次。
func flushLoop() {
	const batchSize = 100
	batch := make([]*ApiLog, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if db := config.GetDB(); db != nil {
			if err := db.CreateInBatches(batch, batchSize).Error; err != nil {
				slog.Error("api 日志批量落库失败", "err", err)
			}
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
