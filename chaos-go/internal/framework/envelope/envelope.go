package envelope

import (
	"bytes"
	"chaos-go/pkg/tools"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/framework/web"
)

type contextKey string

const (
	ctxRequestID contextKey = "envelope.requestID"
	ctxAction    contextKey = "envelope.action"
	ctxData      contextKey = "envelope.data"
	ctxMeta      contextKey = "envelope.meta"
)

type Request struct {
	RequestID string          `json:"requestId"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
	Meta      json.RawMessage `json:"meta"`
	Timestamp int64           `json:"timestamp"`
}

type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"requestId,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

func Middleware() web.HandlerFunc {
	return func(c *web.Context) {
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

					c.Request.Body = io.NopCloser(bytes.NewReader(body))
				}
			}
		}
		c.Next()
	}
}

func Bind(c *web.Context, dst interface{}) error {
	if raw, ok := c.Get(string(ctxData)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			return json.Unmarshal(rawMsg, dst)
		}
	}
	return c.ShouldBindJSON(dst)
}

func GetMeta(c *web.Context, dst interface{}) bool {
	if raw, ok := c.Get(string(ctxMeta)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			if err := json.Unmarshal(rawMsg, dst); err == nil {
				return true
			}
		}
	}
	return false
}

func GetAction(c *web.Context) string {
	if v, ok := c.Get(string(ctxAction)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func ID(c *web.Context) (int, bool) {
	if raw, ok := c.Get(string(ctxData)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			var m struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(rawMsg, &m) == nil && m.ID != 0 {
				return m.ID, true
			}
		}
	}
	if v := c.Param("id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			return id, true
		}
	}
	return 0, false
}

func GetMetaMap(c *web.Context) map[string]any {
	out := map[string]any{}
	if raw, ok := c.Get(string(ctxMeta)); ok {
		if rawMsg, ok := raw.(json.RawMessage); ok && len(rawMsg) > 0 {
			_ = json.Unmarshal(rawMsg, &out)
		}
	}
	return out
}

func MetaToQuery(c *web.Context) {
	m := GetMetaMap(c)
	if len(m) == 0 {
		return
	}
	q := c.Request.URL.Query()
	for k, v := range m {
		key := k
		if key == "pageSize" {
			key = "page_size"
		}
		switch val := v.(type) {
		case string:
			if val != "" {
				q.Set(key, val)
			}
		case float64:
			q.Set(key, strconv.Itoa(int(val)))
		case bool:
			q.Set(key, strconv.FormatBool(val))
		case nil:

		default:
			q.Set(key, fmt.Sprintf("%v", val))
		}
	}
	c.Request.URL.RawQuery = q.Encode()
}

func RequestID(c *web.Context) string {
	if v, ok := c.Get(string(ctxRequestID)); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	id := tools.UUID()
	c.Set(string(ctxRequestID), id)
	return id
}

func Timestamp() int64 {
	return time.Now().UnixMilli()
}
