package envelope_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	renv "chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

// setup 构造一个仅用于验证信封行为的测试路由：挂载 envelope.Middleware，
// handler 从信封取 data/meta/action 并原样回显。
func setup() *web.Router {
	
	r := web.NewRouter()
	g := r.Group("/api/v1/echo")
	g.Use(renv.Middleware())
	g.POST("/bind", func(c *web.Context) {
		var body map[string]any
		if err := renv.Bind(c, &body); err != nil {
			resp.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		meta := map[string]any{}
		renv.GetMeta(c, &meta)
		resp.Success(c, map[string]any{
			"data":   body,
			"meta":   meta,
			"action": renv.GetAction(c),
		})
	})
	return r
}

func TestEnvelopeBind(t *testing.T) {
	r := setup()
	payload := renv.Request{
		RequestID: "req-123",
		Action:    "echo.bind",
		Data:      json.RawMessage(`{"name":"李四"}`),
		Meta:      json.RawMessage(`{"page":1}`),
		Timestamp: 1710000000000,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/echo/bind", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
		Timestamp int64  `json:"timestamp"`
		Data      struct {
			Data   map[string]any `json:"data"`
			Meta   map[string]any `json:"meta"`
			Action string         `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Code != 0 || out.Message != "ok" {
		t.Fatalf("bad envelope: %+v", out)
	}
	if out.RequestID != "req-123" {
		t.Fatalf("requestId want req-123 got %q", out.RequestID)
	}
	if out.Timestamp == 0 {
		t.Fatalf("timestamp missing")
	}
	if out.Data.Action != "echo.bind" {
		t.Fatalf("action want echo.bind got %q", out.Data.Action)
	}
	if out.Data.Data["name"] != "李四" {
		t.Fatalf("data not bound: %+v", out.Data.Data)
	}
	if out.Data.Meta["page"] != float64(1) {
		t.Fatalf("meta not bound: %+v", out.Data.Meta)
	}
}

// TestEnvelopeLegacyFallback 验证无信封的存量请求：Bind 直接绑定整个 body，
// 响应由框架生成服务端 requestId，从而双轨兼容。
func TestEnvelopeLegacyFallback(t *testing.T) {
	r := setup()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/echo/bind", bytes.NewReader([]byte(`{"name":"张三"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		RequestID string `json:"requestId"`
		Data      struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Data.Data["name"] != "张三" {
		t.Fatalf("legacy bind failed: %+v", out.Data.Data)
	}
	if out.RequestID == "" {
		t.Fatalf("legacy request should get server-generated requestId")
	}
}
