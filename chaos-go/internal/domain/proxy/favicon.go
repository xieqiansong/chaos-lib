package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"chaos-go/internal/framework/datacache"
	"chaos-go/internal/framework/memcache"

	"chaos-go/internal/framework/web"
	"golang.org/x/net/html"
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

	// browserUA 模拟主流浏览器 UA 与 Accept，避免站点/WAF 以「无 UA / 爬虫」为由返回 403 或 HTML 错误页
	// （这正是很多站点浏览器能显示、本接口却抓不到的主因）。
	browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
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

// safeHostCache 缓存 safeHost 的可达性校验结果（含否定结果），避免重定向逐跳与回源路径重复做 DNS 查询。
// 进程内、带 TTL；仅做结果复用，不改变校验语义。
var safeHostCache = memcache.New(1024, 5*time.Minute)

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
func GetFavicon(c *web.Context) {
	host := normalizeHost(c.Param("host"))
	if host == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	// 1) 内存级短期缓存（memcache，TTL 5 分钟）第一道关卡：命中即返回，跳过后续校验与回源
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

	// 3) 缓存未命中：做可达性校验，再按序回源（代理 / 直连双跑，并行竞争）
	if !safeHost(host) {
		c.Status(http.StatusBadRequest)
		return
	}
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

// looksLikeImage 用文件头 magic bytes 判断字节是否像图标图片（不依赖 Content-Type 字符串），
// 规避服务端回错误页却标成 image/*、或把 SVG 标成 text/plain 导致的误判/漏判。
func looksLikeImage(b []byte) bool {
	return sniffImageType(b) != ""
}

// sniffImageType 返回能识别出的图片类型（空串表示不像图片）。覆盖站点常见 favicon 格式：
// PNG / GIF / JPEG / WebP / ICO / BMP / SVG。
func sniffImageType(b []byte) string {
	if len(b) < 4 {
		return ""
	}
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		return "image/gif"
	case b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("RIFF")) && len(b) >= 12 && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp"
	case b[0] == 0x00 && b[1] == 0x00 && b[2] == 0x01 && b[3] == 0x00:
		return "image/x-icon" // ICO
	case b[0] == 0x42 && b[1] == 0x4D:
		return "image/bmp"
	case isSVG(b):
		return "image/svg+xml"
	}
	return ""
}

// isSVG 判断字节是否为 SVG（允许前置空白与 XML 声明）。
func isSVG(b []byte) bool {
	s := bytes.TrimSpace(b)
	if len(s) == 0 {
		return false
	}
	if bytes.HasPrefix(s, []byte("<?xml")) {
		s = bytes.TrimSpace(s[5:])
	}
	return bytes.HasPrefix(bytes.ToLower(s), []byte("<svg"))
}

// fetchFavicon 两阶段溯源：
// 阶段一（廉价）：第三方图标服务 + 站点常见固定路径，直连/代理双跑并发竞争，首个图标即返回；
// 阶段二（回退）：解析站点首页 HTML 的 <link rel=icon>/<link rel=manifest>，按发现的真实
// 图标地址再试——这正是浏览器定位 favicon 的方式，可覆盖「无固定 /favicon.ico」的现代站点。
func fetchFavicon(ctx context.Context, hostPort string) ([]byte, string, error) {
	host := hostnameOnly(hostPort)

	if body, ct, err := raceFetch(ctx, buildStaticTasks(host, hostPort)); err == nil {
		return body, ct, nil
	}

	if tasks := discoverFaviconTasks(ctx, hostPort); len(tasks) > 0 {
		if body, ct, err := raceFetch(ctx, tasks); err == nil {
			return body, ct, nil
		}
	}
	return nil, "", fmt.Errorf("favicon: 所有回源均失败 host=%s", hostPort)
}

// raceFetch 并发抓取候选 URL，首个「看起来是图标」的成功结果即返回；其余任务被取消。
func raceFetch(ctx context.Context, tasks []fetchTask) ([]byte, string, error) {
	if len(tasks) == 0 {
		return nil, "", fmt.Errorf("favicon: 无候选源")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type hit struct {
		body []byte
		ct   string
	}
	ch := make(chan hit, 1) // 缓冲 1：首个成功写入，后续自动丢弃
	var wg sync.WaitGroup
	var mu sync.Mutex
	var lastErr error
	for _, t := range tasks {
		wg.Add(1)
		go func(t fetchTask) {
			defer wg.Done()
			b, err := fetchBytes(ctx, t.client, t.url)
			if err != nil {
				mu.Lock()
				if lastErr == nil {
					lastErr = err
				}
				mu.Unlock()
				return
			}
			if len(b) == 0 || !looksLikeImage(b) {
				// 空响应或非图片（如源站回退的 HTML 错误页 / 纯文本）：不夺标，
				// 不取消其余任务，交由其它源/路径继续尝试。
				return
			}
			cancel() // 首个成功夺标，作废其余任务
			select {
			case ch <- hit{b, detectType(b)}:
			default:
			}
		}(t)
	}
	wg.Wait()
	select {
	case h := <-ch:
		return h.body, h.ct, nil
	default:
		if lastErr != nil {
			return nil, "", lastErr
		}
		return nil, "", fmt.Errorf("favicon: 所有回源均失败")
	}
}

// buildStaticTasks 组装「廉价源」任务：第三方图标服务 + 站点常见固定路径（含 .ico/.png/.svg）。
// 每源至少一条直连任务；该源在环境中走代理时，再追加一条代理任务。
func buildStaticTasks(host, hostPort string) []fetchTask {
	urls := []string{
		"https://icons.duckduckgo.com/ip3/" + host + ".ico",
		"https://www.google.com/s2/favicons?domain=" + url.QueryEscape(host) + "&sz=64",
		"https://" + hostPort + "/favicon.ico",
		"https://" + hostPort + "/favicon.png",
		"https://" + hostPort + "/favicon.svg",
		"https://" + hostPort + "/apple-touch-icon.png",
		"https://" + hostPort + "/icon.png",
		"https://" + hostPort + "/static/favicon.ico",
		"https://" + hostPort + "/static/img/favicon.png",
		"http://" + hostPort + "/favicon.ico",
	}
	tasks := make([]fetchTask, 0, len(urls)*2)
	for _, u := range urls {
		tasks = append(tasks, fetchTask{u, faviconDirectClient})
		if usesProxy(u) {
			tasks = append(tasks, fetchTask{u, faviconProxyClient})
		}
	}
	return tasks
}

// discoverFaviconTasks 回退方案：抓取站点首页 HTML，提取 <link rel="icon"> / apple-touch-icon
// 与 <link rel="manifest">（再解析 manifest 的 icons 数组）中的真实图标地址。
// 仅当地址通过 safeHost 校验才入候选；上限 12 避免无界膨胀。
func discoverFaviconTasks(ctx context.Context, hostPort string) []fetchTask {
	page, base := fetchSiteRoot(ctx, hostPort)
	if page == "" || base == nil {
		return nil
	}
	linkIcons, manifestURL := parseIconCandidates(page, base)
	cand := make([]string, 0, 16)
	cand = append(cand, linkIcons...)
	if manifestURL != "" {
		cand = append(cand, fetchManifestIcons(ctx, manifestURL)...)
	}

	seen := make(map[string]struct{}, len(cand))
	tasks := make([]fetchTask, 0, len(cand))
	for _, u := range cand {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		pu, err := url.Parse(u)
		if err != nil || pu.Host == "" {
			continue
		}
		if !safeHost(pu.Host) {
			continue
		}
		tasks = append(tasks, fetchTask{u, faviconDirectClient})
		if usesProxy(u) {
			tasks = append(tasks, fetchTask{u, faviconProxyClient})
		}
	}
	if len(tasks) > 12 {
		tasks = tasks[:12]
	}
	return tasks
}

// fetchSiteRoot 抓取站点首页（先 https 后 http），返回 HTML 文本与页面基准 URL。
// 仅用于解析 favicon 线索，限制读取体积避免被超大页面拖慢。
func fetchSiteRoot(ctx context.Context, hostPort string) (string, *url.URL) {
	var page string
	for _, scheme := range []string{"https", "http"} {
		raw := scheme + "://" + hostPort + "/"
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
		resp, err := faviconDirectClient.Do(req)
		if err != nil {
			continue
		}
		func() {
			defer resp.Body.Close()
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
			page = string(b)
		}()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		return page, u
	}
	return "", nil
}

// parseIconCandidates 单次解析首页 HTML，提取 rel 含 icon 类的 <link> 绝对地址，
// 以及 <link rel="manifest"> 的 manifest 地址（可能为空）。避免重复遍历 DOM。
func parseIconCandidates(page string, base *url.URL) (icons []string, manifestURL string) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, ""
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "rel":
					rel = strings.ToLower(a.Val)
				case "href":
					href = a.Val
				}
			}
			if href != "" {
				if ref, err := url.Parse(href); err == nil {
					abs := base.ResolveReference(ref).String()
					switch {
					case rel == "manifest":
						manifestURL = abs
					case iconRel(rel):
						icons = append(icons, abs)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return icons, manifestURL
}

// iconRel 判断 link 的 rel 是否为图标类。
func iconRel(rel string) bool {
	for _, tok := range strings.Fields(rel) {
		switch tok {
		case "icon", "shortcut", "apple-touch-icon", "apple-touch-icon-precomposed", "mask-icon", "fluid-icon":
			return true
		}
	}
	return false
}

// fetchManifestIcons 抓取 manifest，解析其 icons 数组，返回绝对地址。
func fetchManifestIcons(ctx context.Context, manifestURL string) []string {
	cl := faviconDirectClient
	if usesProxy(manifestURL) {
		cl = faviconProxyClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/manifest+json,application/json,*/*;q=0.8")
	resp, err := cl.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil
	}
	var m struct {
		Icons []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	mb, _ := url.Parse(manifestURL)
	var out []string
	for _, ic := range m.Icons {
		if ic.Src == "" {
			continue
		}
		ref, err := url.Parse(ic.Src)
		if err != nil {
			continue
		}
		out = append(out, mb.ResolveReference(ref).String())
	}
	return out
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
	// 模拟浏览器请求头，避免被站点/WAF 以「无 UA / 爬虫」拦截，返回 403 或 HTML 错误页。
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
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

// detectType 识别图片 Content-Type。优先用文件头 magic bytes（涵盖 SVG / WebP 等
// http.DetectContentType 不识别的格式），兜底再用标准探测；ICO 常被判为 octet-stream，转成 image/x-icon。
func detectType(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if t := sniffImageType(body); t != "" {
		return t
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
// 结果带 TTL 缓存（见 safeHostCache），避免重定向逐跳与回源路径重复做 DNS 查询。
func safeHost(host string) bool {
	h := host
	if hp, _, err := net.SplitHostPort(host); err == nil {
		h = hp
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return false
	}
	// 命中缓存：非空值=历史校验通过，空值=历史校验失败（负缓存），均直接复用
	if v, ok := safeHostCache.Get(faviconCategory, h); ok {
		return len(v) > 0
	}
	ok := lookupHost(h)
	if ok {
		safeHostCache.Set(faviconCategory, h, []byte{1})
	} else {
		safeHostCache.Set(faviconCategory, h, nil)
	}
	return ok
}

// lookupHost 执行实际的 DNS 可达性校验，结果由 safeHost 包裹缓存。
func lookupHost(h string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, h)
	return err == nil && len(addrs) > 0
}
