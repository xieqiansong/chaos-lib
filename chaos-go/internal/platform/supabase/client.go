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

	"chaos-go/internal/framework/config"
)

const (
	defaultTimeout = 15 * time.Second

	maxErrorBody = 512

	restPath = "/rest/v1/"

	preferReturn = "return=representation"

	preferUpsert = "resolution=merge-duplicates," + preferReturn

	defaultSchema = "public"
)

var nonFilterParams = map[string]struct{}{
	"select":      {},
	"order":       {},
	"limit":       {},
	"offset":      {},
	"columns":     {},
	"on_conflict": {},
}

type Client struct {
	baseURL   string
	apiKey    string
	schema    string
	allowlist map[string]struct{}
	hc        *http.Client
}

// baseURL 形如 https://<project-ref>.supabase.co；schema 为空时按 public 处理；
// allowlist 为允许访问的表名清单，为空时任何表名都会被拒绝；timeout <= 0 时取默认值。
func New(baseURL, apiKey, schema string, allowlist []string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("supabase: baseURL is required")
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {

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

type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("supabase: %s %s -> %d: %s", e.Method, e.Path, e.Status, e.Body)
}

type TransportError struct {
	Method string
	Path   string
	Err    error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("supabase: %s %s transport error: %v", e.Method, e.Path, e.Err)
}

func (e *TransportError) Unwrap() error { return e.Err }

func (c *Client) Select(table string, query url.Values, out any) error {
	return c.do(http.MethodGet, table, query, nil, "", out)
}

func (c *Client) Insert(table string, rows any, out any) error {
	return c.do(http.MethodPost, table, nil, rows, preferReturn, out)
}

func (c *Client) Upsert(table, onConflict string, rows any, out any) error {
	query := url.Values{"on_conflict": {onConflict}}
	return c.do(http.MethodPost, table, query, rows, preferUpsert, out)
}

func (c *Client) Update(table string, query url.Values, patch any, out any) error {
	if err := requireFilter(query); err != nil {
		return err
	}
	return c.do(http.MethodPatch, table, query, patch, preferReturn, out)
}

func (c *Client) Delete(table string, query url.Values, out any) error {
	if err := requireFilter(query); err != nil {
		return err
	}
	return c.do(http.MethodDelete, table, query, nil, preferReturn, out)
}

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

	req.Header.Set("apikey", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if prefer != "" {
		req.Header.Set("Prefer", prefer)
	}

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

func (c *Client) allowed(table string) bool {
	_, ok := c.allowlist[table]
	return ok
}

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
