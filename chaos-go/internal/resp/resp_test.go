package resp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

// serve 在隔离的 gin 上下文中执行 handler 并返回响应录制器。
func serve(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	h(c)
	return w
}

func TestSuccessEnvelope(t *testing.T) {
	w := serve(t, func(c *gin.Context) { Success(c, map[string]int{"id": 7}) })

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 期望 200，实际 %d", w.Code)
	}
	assertEnvelope(t, w.Body.Bytes(), map[string]any{
		"code":    float64(0),
		"message": "ok",
		"data":    map[string]any{"id": float64(7)},
	})
}

func TestSuccessMsgUsesCustomMessage(t *testing.T) {
	w := serve(t, func(c *gin.Context) { SuccessMsg(c, "已恢复", nil) })

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 期望 200，实际 %d", w.Code)
	}
	assertEnvelope(t, w.Body.Bytes(), map[string]any{
		"code":    float64(0),
		"message": "已恢复",
		"data":    nil,
	})
}

func TestErrorKeepsHTTPStatusAndCodeAligned(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantHTTP int
	}{
		{"404 原样透传", http.StatusNotFound, http.StatusNotFound},
		{"500 原样透传", http.StatusInternalServerError, http.StatusInternalServerError},
		{"低于 400 一律夹紧到 400", http.StatusOK, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(t, func(ctx *gin.Context) { Error(ctx, tc.status, "记录不存在") })
			if w.Code != tc.wantHTTP {
				t.Fatalf("HTTP 期望 %d，实际 %d", tc.wantHTTP, w.Code)
			}
			var got map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("响应不是合法 JSON: %v", err)
			}
			if got["code"] != float64(tc.wantHTTP) {
				t.Fatalf("code 期望与 HTTP 状态一致（%d），实际 %v", tc.wantHTTP, got["code"])
			}
			if got["message"] != "记录不存在" {
				t.Fatalf("message 期望「记录不存在」，实际 %v", got["message"])
			}
			if _, ok := got["data"]; ok {
				t.Fatal("错误信封不应带 data 字段")
			}
		})
	}
}

// assertEnvelope 逐个字段比对信封，避免「多了/少了字段」被放过。
func assertEnvelope(t *testing.T, body []byte, want map[string]any) {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\nbody=%s", err, body)
	}
	if len(got) != len(want) {
		t.Fatalf("信封字段数期望 %d，实际 %d：%s", len(want), len(got), body)
	}
	for k, wantVal := range want {
		gotVal, ok := got[k]
		if !ok {
			t.Fatalf("信封缺少字段 %s：%s", k, body)
		}
		if !reflect.DeepEqual(gotVal, wantVal) {
			t.Fatalf("字段 %s 期望 %#v，实际 %#v", k, wantVal, gotVal)
		}
	}
}
