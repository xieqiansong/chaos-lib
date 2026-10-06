// Package envelope 定义「POST + Action」统一接口的请求/响应信封，并提供一个 gin 中间件
// 与绑定辅助函数，是接口重构（详见仓库根《接口规范.md》）的底座。
//
// 设计目标（双轨兼容）：
//   - 新接口全部走 POST /api/v1/{module}/{action}，请求体套统一信封；
//   - 存量 /api 接口保持不变，信封中间件只作用于挂载了它的 /api/v1 路由组；
//   - handler 用本包的 Bind / GetMeta 收参，无论新旧请求都能用同一行代码，
//     从而支持模块逐个迁移、随时回退，期间服务始终可运行。
package envelope

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
)

// contextKey 用于在 gin.Context 中暂存信封解析结果。
type contextKey string

const (
	ctxRequestID contextKey = "envelope.requestID"
	ctxAction    contextKey = "envelope.action"
	ctxData      contextKey = "envelope.data"
	ctxMeta      contextKey = "envelope.meta"
)

// Request 是统一的请求信封。业务数据放在 Data 字段，分页/排序/来源等放在 Meta，
// Action 用于审计与校验（URL 已带动作时可省略，但建议保留）。
//
//	{
//	  "requestId": "uuid",
//	  "action": "user.create",
//	  "data": { ... 业务数据 ... },
//	  "meta": { ... 分页/排序/来源 ... },
//	  "timestamp": 1710000000000
//	}
type Request struct {
	RequestID string          `json:"requestId"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
	Meta      json.RawMessage `json:"meta"`
	Timestamp int64           `json:"timestamp"`
}

// Response 是统一的响应信封。Code 成功为 0；Message 成功为 "ok"；
// RequestID 与 Timestamp 由框架自动填充，便于链路追踪与幂等。
//
//	{ "code": 0, "message": "ok", "data": {}, "requestId": "uuid", "timestamp": 1710000000000 }
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"requestId,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// Middleware 解析请求信封，把 data/meta/action/requestId 暂存到 gin.Context，
// 供 Bind / GetMeta / GetAction / RequestID 读取。仅作用于挂载了本中间件的路由组
// （即新的 /api/v1 组），存量 /api 路由不受影响。
//
// 若请求体不是合法信封（无 data 字段），则按存量请求处理：把原始 body 还原，
// 留由 handler 自行 ShouldBindJSON，同时生成 requestId 以便响应回填。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			ct := c.GetHeader("Content-Type")
			if strings.Contains(ct, "application/json") {
				body, err := io.ReadAll(c.Request.Body)
				_ = c.Request.Body.Close()
				if err == nil {
					var req Request
					if json.Unmarshal(body, &req) == nil && len(req.Data) > 0 {
						c.Set(string(ctxData), req.Data)
						c.Set(string(ctxMeta), req.Meta)
						c.Set(string(ctxAction), req.Action)
						if req.RequestID != "" {
							c.Set(string(ctxRequestID), req.RequestID)
						}
					}
					// 还原 body，兼容同一路由下 handler 直接 ShouldBindJSON 的存量写法
					c.Request.Body = io.NopCloser(bytes.NewReader(body))
				}
			}
		}
		c.Next()
	}
}

// Bind 把请求体绑定到 dst：优先解析信封内的 data，否则按存量写法直接绑定整个 body。
// handler 无论新旧请求都能用同一行代码收参。
func Bind(c *gin.Context, dst interface{}) error {
	if raw, ok := c.Get(string(ctxData)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			return json.Unmarshal(rawMsg, dst)
		}
	}
	return c.ShouldBindJSON(dst)
}

// GetMeta 把信封 meta 绑定到 dst（如分页、排序、来源）。非信封请求返回 false。
func GetMeta(c *gin.Context, dst interface{}) bool {
	if raw, ok := c.Get(string(ctxMeta)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			if err := json.Unmarshal(rawMsg, dst); err == nil {
				return true
			}
		}
	}
	return false
}

// GetAction 返回请求中的 action（用于审计/校验），无则返回空串。
func GetAction(c *gin.Context) string {
	if v, ok := c.Get(string(ctxAction)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RequestID 返回本次请求的 requestId：优先取信封携带值，否则生成一个并暂存到 Context，
// 确保响应信封始终能回填 requestId（存量请求也会因此获得一个服务端生成的 id）。
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(string(ctxRequestID)); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	id := uuid.NewString()
	c.Set(string(ctxRequestID), id)
	return id
}

// Timestamp 返回当前毫秒时间戳，供响应信封填充。
func Timestamp() int64 {
	return time.Now().UnixMilli()
}
