// Package supabase 提供对 Supabase Data API（PostgREST）的最小访问能力。
//
// 三条硬约束（对应 openspec/changes/supabase-http-api）：
//  1. 凭据只通过 apikey 请求头传递。Supabase 新式凭据（sb_publishable_* /
//     sb_secret_*）不是 JWT，放进 Authorization: Bearer 会被平台判为 Invalid JWT。
//  2. 只允许访问显式登记的表名，未命中的表名在发出请求前即被拒绝。
//  3. 更新与删除必须携带过滤条件，避免整表被改写或清空。
package supabase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"chaos-go/config"
)

const (
	// defaultTimeout 单次请求超时。免费套餐项目会休眠，冷启动较慢，留出足够余量。
	defaultTimeout = 15 * time.Second
	// maxErrorBody 错误信息中保留的服务端响应体上限，避免日志被大响应体撑爆。
	maxErrorBody = 512
	// restPath 是 PostgREST 在 Supabase 上的固定前缀。
	restPath = "/rest/v1/"
	// preferReturn 要求服务端在响应体中返回被写入 / 删除的行。
	preferReturn = "return=representation"
	// preferUpsert 冲突时合并（存在则更新），并返回写入后的行。
	preferUpsert = "resolution=merge-duplicates," + preferReturn
	// defaultSchema 未显式指定 schema 时使用。
	defaultSchema = "public"
)

// nonFilterParams 是「非过滤」查询参数：查询中只出现这些参数时视为未携带过滤条件。
var nonFilterParams = map[string]struct{}{
	"select":      {},
	"order":       {},
	"limit":       {},
	"offset":      {},
	"columns":     {},
	"on_conflict": {},
}

// Client 是 Supabase Data API（PostgREST）客户端。请用 New 或 Get 获取实例。
type Client struct {
	baseURL   string
	apiKey    string
	schema    string
	allowlist map[string]struct{}
	hc        *http.Client
}

// New 构造客户端。
//
// baseURL 形如 https://<project-ref>.supabase.co；schema 为空时按 public 处理；
// allowlist 为允许访问的表名清单，为空时任何表名都会被拒绝；timeout <= 0 时取默认值。
func New(baseURL, apiKey, schema string, allowlist []string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("supabase: baseURL is required")
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		// 不回显 baseURL：错误信息不得携带配置值
		return nil, errors.New("supabase: baseURL must be an absolute http(s) URL")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("supabase: apiKey is required")
	}
	if schema = strings.TrimSpace(schema); schema == "" {
		schema = defaultSchema
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	allowed := make(map[string]struct{}, len(allowlist))
	for _, table := range allowlist {
		if t := strings.TrimSpace(table); t != "" {
			allowed[t] = struct{}{}
		}
	}

	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		schema:    schema,
		allowlist: allowed,
		hc:        &http.Client{Timeout: timeout},
	}, nil
}

var (
	defaultOnce sync.Once
	defaultCli  *Client
)

// Get 返回由配置驱动的包级单例；配置不完整时返回 nil，调用方必须判空。
// 用法与 config.GetDB() 对称：直接取单例，不搞依赖注入。
func Get() *Client {
	defaultOnce.Do(func() {
		cfg := config.GetConfig().Supabase
		if !cfg.Available() {
			slog.Info("Supabase 数据通道未启用", "reason", "配置不完整")
			return
		}
		cli, err := New(cfg.URL, cfg.SecretKey, cfg.Schema, cfg.Tables,
			time.Duration(cfg.TimeoutSec)*time.Second)
		if err != nil {
			slog.Error("Supabase 客户端初始化失败", "err", err)
			return
		}
		defaultCli = cli
		slog.Info("Supabase 数据通道已启用", "schema", cfg.Schema, "tables", cfg.Tables)
	})
	return defaultCli
}

// APIError 表示服务端返回了非成功状态码。
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("supabase: %s %s -> %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// TransportError 表示请求未能抵达服务端（网络不可达、TLS 失败、超时等）。
// 它与 APIError 区分开，便于调用方判断是否值得重试。
type TransportError struct {
	Method string
	Path   string
	Err    error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("supabase: %s %s transport error: %v", e.Method, e.Path, e.Err)
}

// Unwrap 让 errors.Is / errors.As 能穿透到原始网络错误。
func (e *TransportError) Unwrap() error { return e.Err }

// Select 读取 table 中满足 query 的行。query 支持列选择、过滤、排序与条数上限等。
// 结果为空集时不视为错误。
func (c *Client) Select(table string, query url.Values, out any) error {
	return c.do(http.MethodGet, table, query, nil, "", out)
}

// Insert 插入一行或多行，并返回写入后的行。
func (c *Client) Insert(table string, rows any, out any) error {
	return c.do(http.MethodPost, table, nil, rows, preferReturn, out)
}

// Upsert 按 onConflict 指定的冲突列执行「存在则更新、不存在则插入」，并返回写入后的行。
func (c *Client) Upsert(table, onConflict string, rows any, out any) error {
	query := url.Values{"on_conflict": {onConflict}}
	return c.do(http.MethodPost, table, query, rows, preferUpsert, out)
}

// Update 更新 table 中满足 query 的行，并返回更新后的行。
// query 必须至少包含一个过滤条件，否则返回错误且不发出任何请求。
func (c *Client) Update(table string, query url.Values, patch any, out any) error {
	if err := requireFilter(query); err != nil {
		return err
	}
	return c.do(http.MethodPatch, table, query, patch, preferReturn, out)
}

// Delete 删除 table 中满足 query 的行，并返回被删除的行。
// query 必须至少包含一个过滤条件，否则返回错误且不发出任何请求。
func (c *Client) Delete(table string, query url.Values, out any) error {
	if err := requireFilter(query); err != nil {
		return err
	}
	return c.do(http.MethodDelete, table, query, nil, preferReturn, out)
}

// do 是唯一的请求出口：负责表名校验、请求构造、头部设置、状态码与错误转换。
func (c *Client) do(method, table string, query url.Values, body any, prefer string, out any) error {
	if !c.allowed(table) {
		return fmt.Errorf("supabase: table %q is not in the allowlist", table)
	}

	path := table
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("supabase: marshal request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, c.baseURL+restPath+path, reader)
	if err != nil {
		return fmt.Errorf("supabase: build request for %s %s: %w", method, path, err)
	}

	// 凭据只放 apikey：新式 key 不是 JWT，放进 Authorization 会被判 Invalid JWT。
	req.Header.Set("apikey", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}
	// 非 public schema 必须显式声明：读用 Accept-Profile，写用 Content-Profile。
	if c.schema != defaultSchema {
		if method == http.MethodGet {
			req.Header.Set("Accept-Profile", c.schema)
		} else {
			req.Header.Set("Content-Profile", c.schema)
		}
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return &TransportError{Method: method, Path: path, Err: err}
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &APIError{
			Method: method,
			Path:   path,
			Status: resp.StatusCode,
			Body:   truncate(strings.TrimSpace(string(raw)), maxErrorBody),
		}
	}
	if readErr != nil {
		return &TransportError{Method: method, Path: path, Err: readErr}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("supabase: decode response of %s %s: %w", method, path, err)
		}
	}
	return nil
}

// allowed 判断表名是否在允许清单内。
func (c *Client) allowed(table string) bool {
	_, ok := c.allowlist[table]
	return ok
}

// requireFilter 确保查询中至少存在一个非空的过滤条件。
func requireFilter(query url.Values) error {
	for key, values := range query {
		if _, reserved := nonFilterParams[key]; reserved {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return nil
			}
		}
	}
	return errors.New("supabase: refusing to run without a filter condition")
}

// truncate 按 rune 截断字符串，避免把多字节字符切成两半。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	clipped := s
	for len(clipped) > max {
		_, size := utf8.DecodeLastRuneInString(clipped)
		if size <= 0 {
			break
		}
		clipped = clipped[:len(clipped)-size]
	}
	return clipped + "...(truncated)"
}
