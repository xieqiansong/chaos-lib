package middleware

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

func newTestRouter(t *testing.T, token, deny string) *web.Router {

	if token != "" {
		t.Setenv("CHAOS_API_TOKEN", token)
	} else {
		t.Setenv("CHAOS_API_TOKEN", "")
	}
	if deny != "" {
		t.Setenv("CHAOS_API_DENY_ACTIONS", deny)
	} else {
		t.Setenv("CHAOS_API_DENY_ACTIONS", "")
	}
	r := web.NewRouter()
	v1 := r.Group("/api/v1")
	v1.Use(renv.Middleware())
	v1.Use(AuthMiddleware())
	v1.POST("/demo/ping", func(c *web.Context) {
		resp.Success(c, map[string]any{"ok": true})
	})
	return r
}

func doPost(r *web.Router, path, token string) *httptest.ResponseRecorder {
	payload := renv.Request{Action: "demo.ping", Data: json.RawMessage(`{}`)}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeCode(t *testing.T, w *httptest.ResponseRecorder) int {
	var out struct{ Code int }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out.Code
}

func TestAuthDefaultAllow(t *testing.T) {
	r := newTestRouter(t, "", "")
	w := doPost(r, "/api/v1/demo/ping", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want HTTP 200 got %d", w.Code)
	}
	if code := decodeCode(t, w); code != 0 {
		t.Fatalf("want code 0 got %d", code)
	}
}

func TestAuthMissingToken(t *testing.T) {
	r := newTestRouter(t, "secret", "")
	if code := decodeCode(t, doPost(r, "/api/v1/demo/ping", "")); code != resp.CodeUnauthorized {
		t.Fatalf("want 40100 got %d", code)
	}
}

func TestAuthValidToken(t *testing.T) {
	r := newTestRouter(t, "secret", "")
	if code := decodeCode(t, doPost(r, "/api/v1/demo/ping", "secret")); code != 0 {
		t.Fatalf("want 0 got %d", code)
	}
}

func TestAuthDenyAction(t *testing.T) {
	r := newTestRouter(t, "secret", "demo.ping")
	if code := decodeCode(t, doPost(r, "/api/v1/demo/ping", "secret")); code != resp.CodeForbidden {
		t.Fatalf("want 40300 got %d", code)
	}
}
