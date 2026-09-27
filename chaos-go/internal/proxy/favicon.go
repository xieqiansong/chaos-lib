package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chaos-go/internal/datacache"

	"github.com/gin-gonic/gin"
)

// 站点 favicon 代理：前端不再直连第三方图标服务，统一经本接口
// 1) 先查 datacache（缓存优先）→ 2) 未命中/过期则按序回源（DDG→Google→目标站）
// → 3) 成功即以二进制写回并写入缓存。全部失败返回 404（前端 <img @error> 自动隐藏）。

const (
	faviconCategory = "favicon"
	faviconTTL      = 7 * 24 * time.Hour // 缓存 7 天
	maxFaviconSize  = 1 << 20            // 单图标上限 1 MiB
	faviconTimeout  = 8 * time.Second    // 单次抓取超时
	faviconBudget   = 15 * time.Second   // 单次 favicon 请求总预算（代理+直连双跑的最坏界）
)

// 双路径客户端：
//   - faviconProxyClient：默认 Transport（自动遵循环境 HTTP_PROXY/HTTPS_PROXY/NO_PROXY）
//   - faviconDirectClient：强制直连（Proxy=nil，忽略任何代理变量）
// 两者共用 safeRedirectPolicy：跟随重定向但每一跳都要重新过 SSRF 校验，
// 防止 favicon 跳转跨站指向内网/本机地址（如默默跳到 localhost），绕过顶层校验。
var (
	faviconProxyClient = &http.Client{
		Timeout:       faviconTimeout,
		CheckRedirect: safeRedirectPolicy(),
	}

	faviconDirectTransport = func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = nil
		return t
	}()
	faviconDirectClient = &http.Client{
		Timeout:       faviconTimeout,
		Transport:     faviconDirectTransport,
		CheckRedirect: safeRedirectPolicy(),
	}
)

// safeRedirectPolicy 限制重定向次数，并确保每个跳转目标都通过 SSRF 校验。
// 校验失败返回 error——http.Client 会将错误连同原始响应一并返回，
// fetchBytes 命中 err 分支即视为该源失败，落入下一个回源源。
func safeRedirectPolicy() func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		const maxRedirects = 5
		if len(via) >= maxRedirects {
			return fmt.Errorf("favicon: 重定向次数超过上限 %d", maxRedirects)
		}
		if !safeHost(req.URL.Host) {
			return fmt.Errorf("favicon: 重定向目标未通过 SSRF 校验 host=%s", req.URL.Host)
		}
		return nil
	}
}

// GetFavicon GET /api/favicon/:host —— 返回图片二进制流与识别出的 Content-Type。
func GetFavicon(c *gin.Context) {
	host := normalizeHost(c.Param("host"))
	if host == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if !safeHost(host) {
		c.Status(http.StatusBadRequest)
		return
	}

	// 1) 缓存优先：最新一条且未过期则直接返回
	if row, err := datacache.Get(faviconCategory, host); err == nil && len(row.Value) > 0 {
		if row.ExpireAt == nil || row.ExpireAt.After(time.Now()) {
			c.Data(http.StatusOK, detectType(row.Value), row.Value)
			return
		}
	}

	// 2) 按序回源，成功即停；代理 / 直连双跑（并行竞争）
	ctx, cancel := context.WithTimeout(c.Request.Context(), faviconBudget)
	defer cancel()
	body, ct, err := fetchFavicon(ctx, host)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 3) 写入缓存（追加式，TTL 7 天）并返回
	exp := time.Now().Add(faviconTTL)
	_ = datacache.Set(faviconCategory, host, body, ct, "none", &exp)
	c.Data(http.StatusOK, ct, body)
}

// fetchFavicon 对「3 源 × 2 路径（代理/直连）」一次性全部并发，首个成功即返回。
// 不同于逐源串行等待，这里所有任务同时竞争，整体耗时由最快成功路径决定。
func fetchFavicon(ctx context.Context, hostPort string) ([]byte, string, error) {
	// DDG / Google 只认纯域名，需剥掉端口；缓存 key 仍用完整 host:port
	hostOnly := hostnameOnly(hostPort)
	raws := []string{
		fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", hostOnly),
		fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", url.QueryEscape(hostOnly)),
		directFaviconURL(hostPort), // 直达目标站（host 已过 safeHost 校验），带端口+推断协议
	}

	// 3 源 × 2 路径 任务列表：每源尽力包含「代理 + 直连」，只有配置了代理才加代理路径
	type task struct {
		client *http.Client
		raw    string
	}
	var tasks []task
	for _, raw := range raws {
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil); err == nil {
			if p, err := http.ProxyFromEnvironment(req); err == nil && p != nil {
				tasks = append(tasks, task{faviconProxyClient, raw})
			}
		}
		tasks = append(tasks, task{faviconDirectClient, raw})
	}
	if len(tasks) == 0 {
		return nil, "", fmt.Errorf("favicon: 无可用抓取路径 host=%s", hostPort)
	}

	type favResult struct {
		body []byte
		ct   string
		ok   bool
	}
	ch := make(chan favResult, len(tasks))
	for _, t := range tasks {
		go func(client *http.Client, raw string) {
			b, err := fetchBytes(ctx, client, raw)
			ct := ""
			if err == nil {
				ct = detectType(b)
			}
			// 每个任务必发一条结果（含失败），保证主循环按任务数收满、不阻塞
			select {
			case ch <- favResult{body: b, ct: ct, ok: ct != ""}:
			case <-ctx.Done():
			}
		}(t.client, t.raw)
	}
	for range tasks {
		select {
		case r := <-ch:
			if r.ok {
				return r.body, r.ct, nil
			}
			// 该路径失败，继续等其余并发任务
		case <-ctx.Done():
			return nil, "", fmt.Errorf("favicon: 抓取超时/取消 host=%s", hostPort)
		}
	}
	return nil, "", fmt.Errorf("favicon: 所有回源均失败 host=%s", hostPort)
}

// hostnameOnly 去掉端口，返回纯域名（DDG/Google 图标服务不接受端口）。
func hostnameOnly(hostPort string) string {
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		return h
	}
	return hostPort
}

// directFaviconURL 末端直达目标站：显式端口且非 443 视为 http，否则默认 https。
func directFaviconURL(hostPort string) string {
	scheme := "https"
	if _, port, err := net.SplitHostPort(hostPort); err == nil && port != "443" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/favicon.ico", scheme, hostPort)
}

// fetchBytes 带上下文、状态校验与大小上限地抓取原始字节。
func fetchBytes(ctx context.Context, cl *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("favicon: 状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFaviconSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxFaviconSize {
		return nil, fmt.Errorf("favicon: 图标超过大小上限")
	}
	return body, nil
}

// detectType 识别图片 Content-Type；ICO 常被判为 octet-stream，转成 image/x-icon。
func detectType(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	ct := http.DetectContentType(body)
	if ct == "application/octet-stream" {
		return "image/x-icon"
	}
	return ct
}

// normalizeHost 归一化 host[:port]：剥去 scheme/路径，hostname 转小写。解析失败返回空串。
func normalizeHost(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if p := u.Port(); p != "" {
		host = host + ":" + p
	}
	return host
}

// safeHost 校验主机可解析（DNS/字面 IP 能解析出地址即可）。
// 不做内网/公网/本机过滤：个人书签数据由本应用自己维护，需支持内网与本机站点。
func safeHost(host string) bool {
	h := host
	if hp, _, err := net.SplitHostPort(host); err == nil {
		h = hp
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, h)
	if err != nil || len(addrs) == 0 {
		return false
	}
	return true
}