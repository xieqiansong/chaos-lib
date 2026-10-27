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

const (
	faviconCategory = "favicon"
	faviconTTL      = 7 * 24 * time.Hour
	faviconMemTTL   = 5 * time.Minute
	maxFaviconSize  = 1 << 20
	faviconTimeout  = 5 * time.Second
	faviconBudget   = 15 * time.Second

	browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

var (
	faviconProxyClient = &http.Client{Timeout: faviconTimeout, CheckRedirect: redirectPolicy}

	faviconDirectTransport = func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = nil
		return t
	}()
	faviconDirectClient = &http.Client{Timeout: faviconTimeout, Transport: faviconDirectTransport, CheckRedirect: redirectPolicy}
)

var faviconMemCache = memcache.New(1024, faviconMemTTL)

var safeHostCache = memcache.New(1024, 5*time.Minute)

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

func GetFavicon(c *web.Context, host string) {
	host = normalizeHost(host)
	if host == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	if body, ok := faviconMemCache.Get(faviconCategory, host); ok {
		if len(body) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, detectType(body), body)
		return
	}

	if row, err := datacache.Get(faviconCategory, host); err == nil && len(row.Value) > 0 {
		if row.ExpireAt == nil || row.ExpireAt.After(time.Now()) {
			c.Data(http.StatusOK, detectType(row.Value), row.Value)
			faviconMemCache.Set(faviconCategory, host, row.Value)
			return
		}
	}

	if !safeHost(host) {
		c.Status(http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), faviconBudget)
	defer cancel()
	body, ct, err := fetchFavicon(ctx, host)
	if err != nil || len(body) == 0 {

		faviconMemCache.Set(faviconCategory, host, nil)
		c.Status(http.StatusNotFound)
		return
	}

	exp := time.Now().Add(faviconTTL)
	_ = datacache.Set(faviconCategory, host, body, ct, "none", &exp)
	faviconMemCache.Set(faviconCategory, host, body)
	c.Data(http.StatusOK, ct, body)
}

func looksLikeImage(b []byte) bool {
	return sniffImageType(b) != ""
}

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
		return "image/x-icon"
	case b[0] == 0x42 && b[1] == 0x4D:
		return "image/bmp"
	case isSVG(b):
		return "image/svg+xml"
	}
	return ""
}

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
	ch := make(chan hit, 1)
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

				return
			}
			cancel()
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

func iconRel(rel string) bool {
	for _, tok := range strings.Fields(rel) {
		switch tok {
		case "icon", "shortcut", "apple-touch-icon", "apple-touch-icon-precomposed", "mask-icon", "fluid-icon":
			return true
		}
	}
	return false
}

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

type fetchTask struct {
	url    string
	client *http.Client
}

func usesProxy(rawURL string) bool {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	p, err := http.ProxyFromEnvironment(req)
	return err == nil && p != nil
}

func hostnameOnly(hostPort string) string {
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		return h
	}
	return hostPort
}

func fetchBytes(ctx context.Context, cl *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

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

func safeHost(host string) bool {
	h := host
	if hp, _, err := net.SplitHostPort(host); err == nil {
		h = hp
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return false
	}

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

func lookupHost(h string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, h)
	return err == nil && len(addrs) > 0
}
