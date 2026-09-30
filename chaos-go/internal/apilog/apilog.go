// Package apilog 接口访问日志：记录每个 /api 请求的元数据与请求/响应体，
// 作为「标准数据」落库（嵌入 crud.BaseModel，复用标准 CRUD 基线，前端可分页检索）。
//
// 设计要点：
//   - 通过自定义 gin 中间件捕获请求体（读取后回填，保证下游 handler 仍可读取）、
//     响应体（包装 gin.ResponseWriter），以及请求时间、相应用时、状态码等元数据；
//   - 落库走「缓冲通道 + 后台批处理」异步写入，避免每个请求都同步写库造成链路抖动；
//   - 通道满时丢弃当前日志（不阻塞请求），后台每 2s 或累计 100 条触发一次批量落库。
package apilog

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"time"

	"chaos-go/config"
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
	RequestBody  string `gorm:"type:text" json:"RequestBody"`  // 请求体原文
	ResponseBody string `gorm:"type:text" json:"ResponseBody"` // 响应体原文
	BodySize     int    `json:"BodySize"`
	ErrorMsg     string `json:"ErrorMsg"`
}

// TableName 显式指定表名（与前端资源名 apiLog 对应）。
func (ApiLog) TableName() string { return "api_logs" }

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 纯 CRUD 交给通用 crud，前端按路径/方法/错误信息搜索、按耗时/状态排序。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "apiLog", &ApiLog{}, crud.Opts{
		Searchable: []string{"path", "method", "error_msg"},
		Sortable:   []string{"id", "created_at", "latency_ms", "status_code"},
	})
}

// skipBodyPrefixes 命中这些路径前缀时不记录 body（默认空：完整记录）。
// 如需规避敏感/超大 body，可在启动时向此切片追加前缀（如 "/api/envVariables"）。
// 默认含 /api/favicon：该接口返回图标二进制，含 0x00 等字节，落入 text 列会被
// PostgreSQL 以 UTF8 编码错误（SQLSTATE 22021）拒绝，且无文本记录价值。
var skipBodyPrefixes = []string{"/api/favicon"}

// responseWriter 包装 gin.ResponseWriter，在写入下游的同时把响应体缓存到 buf。
type responseWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// logCh 异步落库缓冲通道。
var logCh = make(chan *ApiLog, 2000)

func init() {
	go flushLoop()
}

// Middleware 记录每个 /api 请求的元数据与请求/响应体，异步批量落库。
// 非 /api 路径（静态资源、SPA 回退等）直接放行，不记录。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		start := time.Now()

		// 读取并回填请求体，保证后续 handler 仍可正常读取。
		var reqBody string
		if c.Request.Body != nil {
			if raw, err := io.ReadAll(c.Request.Body); err == nil {
				reqBody = string(raw)
				c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			}
		}

		// 包装响应 writer 以捕获响应体（置于 gzip 等外层 writer 之内，捕获明文）。
		rw := &responseWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = rw

		c.Next()

		respBody := rw.buf.String()
		for _, p := range skipBodyPrefixes {
			if strings.HasPrefix(c.Request.URL.Path, p) {
				reqBody, respBody = "", ""
				break
			}
		}

		rec := &ApiLog{
			Path:         c.Request.URL.Path,
			Method:       c.Request.Method,
			StatusCode:   c.Writer.Status(),
			LatencyMs:    time.Since(start).Milliseconds(),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestBody:  reqBody,
			ResponseBody: respBody,
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
