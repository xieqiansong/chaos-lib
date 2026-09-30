package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"chaos-go/internal/datacache"
	"chaos-go/internal/memcache"

	"github.com/gin-gonic/gin"
)

// 站点 favicon 代理：前端不再直连第三方图标服务，统一经本接口
// 1) 先查 datacache（缓存优先）→ 2) 未命中/过期则按序回源（DDG→Google→目标站）
// → 3) 成功即以二进制写回并写入缓存。全部失败返回 404（前端 <img @error> 自动隐藏）。

const (
	faviconCategory = "favicon"
	faviconTTL      = 7 * 24 * time.Hour // 持久缓存 7 天
	faviconMemTTL   = 5 * time.Minute    // 内存缓存 5 分钟（短期兜底，缓解回源失败）
	maxFaviconSize  = 1 << 20            // 单图标上限 1 MiB
	faviconTimeout  = 5 * time.Second    // 单次抓取超时
	faviconBudget   = 15 * time.Second   // 单次 favicon 请求总预算（代理+直连双跑的最坏界）
)

// 双路径客户端：faviconProxyClient 走代理，faviconDirectClient 强制直连。
// 共用 redirectPolicy：跟随后每跳重定向都要再过一遍 safeHost，防跳转子网/本机绕过校验。
var (
	faviconProxyClient = &http.Client{Timeout: faviconTimeout, CheckRedirect: redirectPolicy}

	faviconDirectTransport = func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = nil
		return t
	}()
	faviconDirectClient = &http.Client{Timeout: faviconTimeout, Transport: faviconDirectTransport, CheckRedirect: redirectPolicy}
)

// faviconMemCache 内存级短期缓存：进程内、重启即失，TTL 5 分钟。
// 用于 datacache 缺失/过期或回源失败时的兜底，缓解溯源等高频重试场景下的回源失败。
var faviconMemCache = memcache.New(1024, faviconMemTTL)

// redirectPolicy 限跳 5 次，并确保每个重定向目标都通过 SSRF 校验。
// 校验失败返回 error——http.Client 连同原始响应一并返回，fetchBytes 命中 err 即该路失败。
func redirectPolicy(req *http.Request, via []*http.Request) error {
	const maxRedirects = 5
	if len(via) >= maxRedirects {
		return fmt.Errorf("favicon: 重定向次数超过上限 %d", maxRedirects)
	}
	if !safeHost(req.URL.Host) {
		return fmt.Errorf("favicon: 重定向目标未通过 SSRF 校验 host=%s", req.URL.Host)
	}
	return nil
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

	// 1) 内存级短期缓存（memcache，TTL 5 分钟）第一道关卡：
	//    - 命中且非空 → 直接返回（含近期成功回源结果，省去回源/查库）
	//    - 命中但为空对象（负缓存，已知回源失败）→ 直接 404，不再回源/查库
	if body, ok := faviconMemCache.Get(faviconCategory, host); ok {
		if len(body) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, detectType(body), body)
		return
	}

	// 2) 持久缓存（datacache，TTL 7 天）：命中且未过期则直接返回
	if row, err := datacache.Get(faviconCategory, host); err == nil && len(row.Value) > 0 {
		if row.ExpireAt == nil || row.ExpireAt.After(time.Now()) {
			c.Data(http.StatusOK, detectType(row.Value), row.Value)
			faviconMemCache.Set(faviconCategory, host, row.Value)
			return
		}
	}

	// 3) 按序回源，成功即停；代理 / 直连双跑（并行竞争）
	ctx, cancel := context.WithTimeout(c.Request.Context(), faviconBudget)
	defer cancel()
	body, ct, err := fetchFavicon(ctx, host)
	if err != nil || len(body) == 0 {
		// 溯源失败（含回源报错或返回空内容）：把空对象写入内存（负缓存 5 分钟），
		// 避免溯源等高频重试短时间反复回源
		faviconMemCache.Set(faviconCategory, host, nil)
		c.Status(http.StatusNotFound)
		return
	}

	// 4) 溯源成功：同时写入持久缓存（TTL 7 天）与内存短期缓存（TTL 5 分钟）并返回
	exp := time.Now().Add(faviconTTL)
	_ = datacache.Set(faviconCategory, host, body, ct, "none", &exp)
	faviconMemCache.Set(faviconCategory, host, body)
	c.Data(http.StatusOK, ct, body)
}

// nonIconContentTypes 反向排查：以下类型为「非图标」，通常是源站未命中 favicon 时
// 回退的 HTML 错误页或纯文本，不应作为图标缓存，需交由其它源/路径继续尝试。
var nonIconContentTypes = map[string]struct{}{
	"text/html; charset=utf-8": {},
	"text/plain; charset=utf-8": {},
}

// isIconType 判断检测出的内容类型是否为有效图标类型（排除已知的非图标类型）。
func isIconType(ct string) bool {
	_, bad := nonIconContentTypes[ct]
	return !bad
}

// fetchFavicon 并发抓取「3 源 × 2 路径（代理/直连）」，首个成功（且类型为图标）即返回。
func fetchFavicon(ctx context.Context, hostPort string) ([]byte, string, error) {
	// DDG / Google 只认纯域名，需剥掉端口；缓存 key 仍用完整 host:port
	host := hostnameOnly(hostPort)
	urls := []string{
		"https://icons.duckduckgo.com/ip3/" + host + ".ico",
		"https://www.google.com/s2/favicons?domain=" + url.QueryEscape(host) + "&sz=64",
		"https://" + hostPort + "/favicon.ico",
		"http://" + hostPort + "/favicon.ico",
	}

	// 组装任务表：每源至少一条直连任务；该源在环境中走代理时，再追加一条代理任务
	tasks := make([]fetchTask, 0, len(urls)*2)
	for _, u := range urls {
		tasks = append(tasks, fetchTask{u, faviconDirectClient})
		if usesProxy(u) {
			tasks = append(tasks, fetchTask{u, faviconProxyClient})
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type hit struct {
		body []byte
		ct   string
	}
	ch := make(chan hit, 1) // 缓冲 1：首个成功写入，后续自动丢弃
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func(t fetchTask) {
			defer wg.Done()
			b, err := fetchBytes(ctx, t.client, t.url)
			if err != nil {
				return // 失败静默，交由 wg 收尾
			}
			if len(b) == 0 {
				return // 空响应非图标，不夺标，让其它源/路径继续尝试
			}
			ct := detectType(b)
			if !isIconType(ct) {
				// 类型非图标（如源站回退的 HTML 错误页 / 纯文本），本次回源视为异常：
				// 不夺标、不取消其余任务，交由其它源/路径继续尝试。
				return
			}
			cancel() // 首个成功夺标，作废其余任务
			select {
			case ch <- hit{b, ct}:
			default:
			}
		}(t)
	}
	wg.Wait()
	select {
	case h := <-ch:
		return h.body, h.ct, nil
	default:
		return nil, "", fmt.Errorf("favicon: 所有回源均失败 host=%s", hostPort)
	}
}

// fetchTask 描述一次回源尝试：目标 URL 与其使用的客户端（代理或直连）。
type fetchTask struct {
	url    string
	client *http.Client
}

// usesProxy 判断该 URL 在当前环境（HTTP_PROXY / NO_PROXY 等）下是否走代理。
func usesProxy(rawURL string) bool {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	p, err := http.ProxyFromEnvironment(req)
	return err == nil && p != nil
}

// hostnameOnly 去掉端口，返回纯域名（DDG/Google 图标服务不接受端口）。
func hostnameOnly(hostPort string) string {
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		return h
	}
	return hostPort
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
