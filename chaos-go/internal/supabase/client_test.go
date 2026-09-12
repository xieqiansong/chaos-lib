package supabase

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const testAPIKey = "sb_secret_test_key_do_not_use"

// captured 记录打桩服务端收到的一次请求。
type captured struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   string
}

// captureServer 启动一个记录请求的打桩服务端；status 为 0 时返回 200。
func captureServer(t *testing.T, status int, respBody string) (*httptest.Server, *[]captured) {
	t.Helper()
	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, captured{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			header: r.Header.Clone(),
			body:   string(raw),
		})
		if status != 0 {
			w.WriteHeader(status)
		}
		if respBody != "" {
			_, _ = w.Write([]byte(respBody))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func newTestClient(t *testing.T, baseURL string, tables ...string) *Client {
	t.Helper()
	if len(tables) == 0 {
		tables = []string{"kv_store"}
	}
	c, err := New(baseURL, testAPIKey, defaultSchema, tables, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func TestNewRejectsInvalidInput(t *testing.T) {
	if _, err := New("", testAPIKey, "", []string{"t"}, 0); err == nil {
		t.Error("New with empty baseURL should fail")
	}
	if _, err := New("ftp://example.com", testAPIKey, "", []string{"t"}, 0); err == nil {
		t.Error("New with non-http(s) baseURL should fail")
	}
	if _, err := New("https://example.com", "   ", "", []string{"t"}, 0); err == nil {
		t.Error("New with blank apiKey should fail")
	}

	c, err := New("https://example.com/", testAPIKey, "  ", nil, 0)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.schema != defaultSchema {
		t.Errorf("schema = %q, want %q", c.schema, defaultSchema)
	}
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
	if c.hc.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.hc.Timeout, defaultTimeout)
	}
}

// 任务 2.2：凭据只放 apikey，绝不出现 Authorization。
func TestCredentialsUseAPIKeyHeaderOnly(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	var rows []map[string]any
	if err := c.Select("kv_store", url.Values{"select": {"*"}}, &rows); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("server received %d requests, want 1", len(*got))
	}

	h := (*got)[0].header
	if h.Get("apikey") != testAPIKey {
		t.Errorf("apikey = %q, want %q", h.Get("apikey"), testAPIKey)
	}
	if v := h.Get("Authorization"); v != "" {
		t.Errorf("Authorization must not be set, got %q", v)
	}
	for key := range h {
		if strings.EqualFold(key, "Authorization") {
			t.Errorf("unexpected auth header %q", key)
		}
	}
	if ct := h.Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type must be omitted when there is no body, got %q", ct)
	}
}

func TestWriteRequestsSetContentType(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	if err := c.Insert("kv_store", map[string]any{"key": "demo"}, nil); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if ct := (*got)[0].header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// 任务 2.3：Select 的路径与查询串拼接。
func TestSelectBuildsPathAndQuery(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	query := url.Values{}
	query.Set("select", "id,key,value")
	query.Set("key", "eq.demo")
	query.Set("order", "id.desc")
	query.Set("limit", "20")

	var rows []map[string]any
	if err := c.Select("kv_store", query, &rows); err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	call := (*got)[0]
	if call.method != http.MethodGet {
		t.Errorf("method = %q, want GET", call.method)
	}
	if call.path != "/rest/v1/kv_store" {
		t.Errorf("path = %q, want /rest/v1/kv_store", call.path)
	}
	for key, want := range map[string]string{
		"select": "id,key,value",
		"key":    "eq.demo",
		"order":  "id.desc",
		"limit":  "20",
	} {
		if v := call.query.Get(key); v != want {
			t.Errorf("query %q = %q, want %q", key, v, want)
		}
	}
}

func TestSelectEmptyResultIsNotAnError(t *testing.T) {
	srv, _ := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	rows := []map[string]any{{"id": float64(1)}}
	if err := c.Select("kv_store", nil, &rows); err != nil {
		t.Fatalf("Select() error = %v, want nil for an empty result set", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %v, want empty", rows)
	}
}

// 任务 2.4：Insert 请求写入后的行。
func TestInsertRequestsRepresentation(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, `[{"id":1,"key":"demo"}]`)
	c := newTestClient(t, srv.URL)

	var out []map[string]any
	if err := c.Insert("kv_store", map[string]any{"key": "demo"}, &out); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	call := (*got)[0]
	if call.method != http.MethodPost {
		t.Errorf("method = %q, want POST", call.method)
	}
	if !strings.Contains(call.header.Get("Prefer"), "return=representation") {
		t.Errorf("Prefer = %q, want it to contain return=representation", call.header.Get("Prefer"))
	}
	if !strings.Contains(call.body, `"key":"demo"`) {
		t.Errorf("body = %q, want it to contain the submitted row", call.body)
	}
	if len(out) != 1 {
		t.Fatalf("decoded %d rows, want 1", len(out))
	}
	if got := out[0]["key"]; got != "demo" {
		t.Errorf("row key = %v, want demo", got)
	}
}

// 任务 2.5：Upsert 指定冲突列并合并。
func TestUpsertUsesConflictColumn(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, `[{"id":1,"key":"demo","value":"v2"}]`)
	c := newTestClient(t, srv.URL)

	var out []map[string]any
	if err := c.Upsert("kv_store", "key", map[string]any{"key": "demo", "value": "v2"}, &out); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	call := (*got)[0]
	if call.method != http.MethodPost {
		t.Errorf("method = %q, want POST", call.method)
	}
	if v := call.query.Get("on_conflict"); v != "key" {
		t.Errorf("on_conflict = %q, want key", v)
	}
	prefer := call.header.Get("Prefer")
	if !strings.Contains(prefer, "resolution=merge-duplicates") {
		t.Errorf("Prefer = %q, want it to contain resolution=merge-duplicates", prefer)
	}
	if !strings.Contains(prefer, "return=representation") {
		t.Errorf("Prefer = %q, want it to also contain return=representation", prefer)
	}
}

// 任务 2.6：Update 必须携带过滤条件。
func TestUpdateRequiresFilter(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	if err := c.Update("kv_store", nil, map[string]any{"value": "x"}, nil); err == nil {
		t.Error("Update without a query should fail")
	}
	if err := c.Update("kv_store", url.Values{"select": {"*"}}, map[string]any{"value": "x"}, nil); err == nil {
		t.Error("Update with only non-filter params should fail")
	}
	if len(*got) != 0 {
		t.Fatalf("server received %d requests, want 0 for rejected updates", len(*got))
	}

	var out []map[string]any
	if err := c.Update("kv_store", url.Values{"key": {"eq.demo"}}, map[string]any{"value": "x"}, &out); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	call := (*got)[0]
	if call.method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", call.method)
	}
	if v := call.query.Get("key"); v != "eq.demo" {
		t.Errorf("filter key = %q, want eq.demo", v)
	}
}

// 任务 2.7：Delete 必须携带过滤条件。
func TestDeleteRequiresFilter(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	if err := c.Delete("kv_store", nil, nil); err == nil {
		t.Error("Delete without a query should fail")
	}
	if err := c.Delete("kv_store", url.Values{"order": {"id.desc"}}, nil); err == nil {
		t.Error("Delete with only non-filter params should fail")
	}
	if len(*got) != 0 {
		t.Fatalf("server received %d requests, want 0 for rejected deletes", len(*got))
	}

	var out []map[string]any
	if err := c.Delete("kv_store", url.Values{"key": {"eq.demo"}}, &out); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	call := (*got)[0]
	if call.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", call.method)
	}
	if !strings.Contains(call.header.Get("Prefer"), "return=representation") {
		t.Errorf("Prefer = %q, want it to contain return=representation", call.header.Get("Prefer"))
	}
}

// 任务 2.8：白名单外的表名必须在发出请求前被拒绝。
func TestTableAllowlistBlocksRequest(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL, "kv_store")

	err := c.Select("other_table", nil, nil)
	if err == nil {
		t.Fatal("Select on a table outside the allowlist should fail")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error = %v, want it to mention the allowlist", err)
	}
	if len(*got) != 0 {
		t.Fatalf("server received %d requests, want 0 for a blocked table", len(*got))
	}
}

func TestEmptyAllowlistRejectsEverything(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")

	empty, err := New(srv.URL, testAPIKey, defaultSchema, nil, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := empty.Select("kv_store", nil, nil); err == nil {
		t.Error("client without an allowlist should reject every table")
	}
	if len(*got) != 0 {
		t.Fatalf("server received %d requests, want 0", len(*got))
	}
}

// 任务 2.9：非 2xx 转成可读的 APIError。
func TestNon2xxBecomesAPIError(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{http.StatusBadRequest, `{"message":"invalid input"}`},
		{http.StatusUnauthorized, `{"message":"Invalid JWT"}`},
		{http.StatusForbidden, `{"message":"new row violates row-level security policy"}`},
		{http.StatusNotFound, `{"code":"PGRST205","message":"table not found"}`},
		{http.StatusInternalServerError, `{"message":"boom"}`},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			srv, _ := captureServer(t, tc.status, tc.body)
			c := newTestClient(t, srv.URL)

			err := c.Select("kv_store", nil, nil)
			if err == nil {
				t.Fatal("Select() error = nil, want a failure")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T, want *APIError", err)
			}
			if apiErr.Status != tc.status {
				t.Errorf("status = %d, want %d", apiErr.Status, tc.status)
			}
			if apiErr.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", apiErr.Method)
			}
			if apiErr.Path != "kv_store" {
				t.Errorf("path = %q, want kv_store", apiErr.Path)
			}
			if apiErr.Body == "" {
				t.Error("Body should carry the server response")
			}

			var transportErr *TransportError
			if errors.As(err, &transportErr) {
				t.Error("a server error status must not be reported as a transport error")
			}
		})
	}
}

// 任务 2.9：网络层失败与「服务端返回错误」可区分。
func TestTransportErrorIsDistinguishable(t *testing.T) {
	srv, _ := captureServer(t, http.StatusOK, "[]")
	baseURL := srv.URL
	srv.Close() // 关闭后连接必被拒绝

	c := newTestClient(t, baseURL)
	err := c.Select("kv_store", nil, nil)
	if err == nil {
		t.Fatal("Select() error = nil, want a transport failure")
	}

	var transportErr *TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("error type = %T, want *TransportError", err)
	}
	if transportErr.Unwrap() == nil {
		t.Error("TransportError should wrap the underlying network error")
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("a transport failure must not be reported as an APIError")
	}
}

// 任务 2.9：错误文本不得泄漏凭据。
func TestErrorNeverLeaksAPIKey(t *testing.T) {
	srv, _ := captureServer(t, http.StatusUnauthorized, "Invalid JWT")
	c := newTestClient(t, srv.URL)

	if err := c.Select("kv_store", nil, nil); err == nil {
		t.Fatal("Select() error = nil, want a failure")
	} else if strings.Contains(err.Error(), testAPIKey) {
		t.Errorf("error text leaks the api key: %v", err)
	}

	if _, err := New("https://example.com", "", "", nil, 0); err == nil {
		t.Fatal("New() with a blank key should fail")
	} else if strings.Contains(err.Error(), "https://example.com") {
		t.Errorf("config error leaks the base URL: %v", err)
	}
}

// 任务 2.2：非 public schema 需要 profile 头。
func TestSchemaProfileHeaders(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c, err := New(srv.URL, testAPIKey, "chaos", []string{"kv_store"}, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := c.Select("kv_store", nil, nil); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	read := (*got)[0].header
	if v := read.Get("Accept-Profile"); v != "chaos" {
		t.Errorf("Accept-Profile = %q, want chaos", v)
	}
	if v := read.Get("Content-Profile"); v != "" {
		t.Errorf("Content-Profile must be omitted on reads, got %q", v)
	}

	if err := c.Insert("kv_store", map[string]any{"key": "demo"}, nil); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	write := (*got)[1].header
	if v := write.Get("Content-Profile"); v != "chaos" {
		t.Errorf("Content-Profile = %q, want chaos", v)
	}
	if v := write.Get("Accept-Profile"); v != "" {
		t.Errorf("Accept-Profile must be omitted on writes, got %q", v)
	}
}

func TestPublicSchemaOmitsProfileHeaders(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, "[]")
	c := newTestClient(t, srv.URL)

	if err := c.Select("kv_store", nil, nil); err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	h := (*got)[0].header
	if v := h.Get("Accept-Profile"); v != "" {
		t.Errorf("Accept-Profile = %q, want it omitted for the public schema", v)
	}
	if v := h.Get("Content-Profile"); v != "" {
		t.Errorf("Content-Profile = %q, want it omitted for the public schema", v)
	}
}

func TestTruncateKeepsUTF8Intact(t *testing.T) {
	got := truncate(strings.Repeat("测", 400), maxErrorBody)
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Errorf("truncate() = %q, want a truncation marker", got)
	}
	if !utf8.ValidString(got) {
		t.Error("truncate() produced invalid UTF-8")
	}
	if len(got) > maxErrorBody+len("...(truncated)") {
		t.Errorf("truncate() length = %d, want it bounded", len(got))
	}
}

// TestIntegrationDataAPILifecycle 在提供真实凭据时才运行，否则跳过。
// 覆盖「插入 → 查询 → 更新 → upsert → 删除 → 确认已删除」完整链路。
func TestIntegrationDataAPILifecycle(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY"))
	table := strings.TrimSpace(os.Getenv("SUPABASE_DEMO_TABLE"))
	if baseURL == "" || apiKey == "" || table == "" {
		t.Skip("set SUPABASE_URL, SUPABASE_SECRET_KEY and SUPABASE_DEMO_TABLE to run the Supabase integration test")
	}

	c, err := New(baseURL, apiKey, os.Getenv("SUPABASE_SCHEMA"), []string{table}, 30*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	key := fmt.Sprintf("chaos-it-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		var removed []map[string]any
		if err := c.Delete(table, url.Values{"key": {"eq." + key}}, &removed); err != nil {
			t.Logf("cleanup delete failed: %v", err)
		}
	})

	// 1. 插入
	var inserted []map[string]any
	if err := c.Insert(table, map[string]any{"key": key, "value": "v1"}, &inserted); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(inserted) != 1 {
		t.Fatalf("insert returned %d rows, want 1", len(inserted))
	}
	if got, _ := inserted[0]["value"].(string); got != "v1" {
		t.Errorf("inserted value = %v, want v1", inserted[0]["value"])
	}

	// 2. 查询
	var found []map[string]any
	if err := c.Select(table, url.Values{"key": {"eq." + key}}, &found); err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("select returned %d rows, want 1", len(found))
	}

	// 3. 更新
	var patched []map[string]any
	if err := c.Update(table, url.Values{"key": {"eq." + key}}, map[string]any{"value": "v2"}, &patched); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(patched) != 1 {
		t.Fatalf("update returned %d rows, want 1", len(patched))
	}
	if got, _ := patched[0]["value"].(string); got != "v2" {
		t.Errorf("updated value = %v, want v2", patched[0]["value"])
	}

	// 4. upsert
	var upserted []map[string]any
	if err := c.Upsert(table, "key", map[string]any{"key": key, "value": "v3"}, &upserted); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(upserted) != 1 {
		t.Fatalf("upsert returned %d rows, want 1", len(upserted))
	}
	if got, _ := upserted[0]["value"].(string); got != "v3" {
		t.Errorf("upserted value = %v, want v3", upserted[0]["value"])
	}

	// 5. 删除并返回被删除的行
	var deleted []map[string]any
	if err := c.Delete(table, url.Values{"key": {"eq." + key}}, &deleted); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("delete returned %d rows, want 1", len(deleted))
	}

	// 6. 确认已删除
	var after []map[string]any
	if err := c.Select(table, url.Values{"key": {"eq." + key}}, &after); err != nil {
		t.Fatalf("select after delete: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("select after delete returned %d rows, want 0", len(after))
	}

	raw, _ := json.Marshal(deleted)
	t.Logf("integration lifecycle ok on table %q with key %q; deleted row: %s", table, key, raw)
}
