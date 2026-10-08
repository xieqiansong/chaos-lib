// Package web 是 net/http 之上的轻量兼容层，用于替代 gin：提供与 gin 近似的
// Context / RouterGroup API，使业务 handler 的迁移尽量只涉及类型改名，而不必重写
// 业务逻辑。它是 chaos-go 自有实现的薄封装，不引入任何第三方 Web 框架。
//
// 设计要点：
//   - 路由基于 Go 1.22+ 的 net/http.ServeMux（支持 "METHOD /path/{name}" 与
//     "GET /assets/{filepath...}" 通配），group 通过拼接前缀实现；
//   - 请求作用域数据（envelope 的 data/meta/action/requestId、Errors）存放在
//     requestState 中，并随 *http.Request 的 context 在全局中间件与路由处理器之间共享；
//   - 响应状态 / 大小 / 客户端 IP 由包装的 responseWriter 记录，供访问日志中间件读取；
//   - 全局中间件（如访问日志）在 Router.ServeHTTP 内以 handler 链形式执行，
//     链尾把请求交给 mux 分发，从而复用同一套 Context 与 writer。
//
// 字段命名对齐 gin：Context.Request / Context.Writer 可直接访问底层 *http.Request 与
// 包装后的响应写入器，便于存量代码平滑迁移。
package web

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// HandlerFunc 与 gin.HandlerFunc 对齐：处理器签名。
type HandlerFunc func(c *Context)

// contextKey 用于把请求级共享状态挂到 *http.Request 的 context 上。
type contextKey string

const stateKey contextKey = "web.requestState"

// requestState 在一次请求内于全局中间件链与路由处理器之间共享的状态。
// 由于二者分别运行在不同的 Context 实例上，所有需要共享的数据都集中放在这里。
type requestState struct {
	rw   *responseWriter
	keys map[string]any
	errs []error
}

// Context 等价于 gin.Context：封装响应写入、请求读取与请求作用域 KV。
// 字段 Request / Writer 与 gin 同名，便于存量代码平滑迁移。
type Context struct {
	// Writer 为包装后的响应写入器，提供 Status()/Size() 等读取能力。
	Writer *responseWriter
	// Request 即原始 *http.Request，可直接访问 URL/Body/Header/Context 等。
	Request *http.Request

	state    *requestState
	handlers []HandlerFunc
	index    int
}

// Next 执行后续处理器（含中间件），实现 handler 链。
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort 终止后续处理器，等价于 gin 的 c.Abort()。
func (c *Context) Abort() {
	c.index = len(c.handlers)
}

// Set 写入请求作用域 KV，等价于 gin 的 c.Set()。
func (c *Context) Set(key string, value any) {
	c.state.keys[key] = value
}

// Get 读取请求作用域 KV，返回值与是否存在，等价于 gin 的 c.Get()。
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.state.keys[key]
	return v, ok
}

// MustGet 读取 KV，缺失时 panic，等价于 gin 的 c.MustGet()。
func (c *Context) MustGet(key string) any {
	v, ok := c.state.keys[key]
	if !ok {
		panic(fmt.Sprintf("web: context missing key %q", key))
	}
	return v
}

// Error 记录一个错误，供访问日志等中间件读取，等价于 gin 的 c.Error()。
func (c *Context) Error(err error) {
	if err != nil {
		c.state.errs = append(c.state.errs, err)
	}
}

// Errors 返回本请求累积的错误列表（只读），等价于 gin 的 c.Errors。
func (c *Context) Errors() []error {
	return c.state.errs
}

// JSON 以 JSON 写回响应并设置 HTTP 状态，等价于 gin 的 c.JSON()。
func (c *Context) JSON(code int, obj any) {
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	_ = json.NewEncoder(c.Writer).Encode(obj)
}

// Data 以原始字节写回响应并设置 Content-Type 与 HTTP 状态，等价于 gin 的 c.Data()。
func (c *Context) Data(code int, contentType string, data []byte) {
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	c.Writer.WriteHeader(code)
	_, _ = c.Writer.Write(data)
}

// Status 设置 HTTP 状态码（惰性写入，重复调用以首次为准），等价于 gin 的 c.Status()。
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

// String 以纯文本写回响应，等价于 gin 的 c.String()。
func (c *Context) String(code int, format string, values ...any) {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	fmt.Fprintf(c.Writer, format, values...)
}

// Param 读取路径参数（{name}），等价于 gin 的 c.Param()。
func (c *Context) Param(name string) string {
	return c.Request.PathValue(name)
}

// Query 读取 URL 查询参数，等价于 gin 的 c.Query()。
func (c *Context) Query(name string) string {
	return c.Request.URL.Query().Get(name)
}

// DefaultQuery 读取 URL 查询参数，缺失时返回默认值，等价于 gin 的 c.DefaultQuery()。
func (c *Context) DefaultQuery(name, def string) string {
	if v := c.Request.URL.Query().Get(name); v != "" {
		return v
	}
	return def
}

// PostForm 读取表单字段，等价于 gin 的 c.PostForm()。
func (c *Context) PostForm(name string) string {
	_ = c.Request.ParseForm()
	return c.Request.PostFormValue(name)
}

// ShouldBindJSON 把请求体 JSON 解码到 dst，等价于 gin 的 c.ShouldBindJSON()。
func (c *Context) ShouldBindJSON(dst any) error {
	return json.NewDecoder(c.Request.Body).Decode(dst)
}

// BindJSON 同 ShouldBindJSON，兼容存量写法。
func (c *Context) BindJSON(dst any) error {
	return c.ShouldBindJSON(dst)
}

// GetHeader 读取请求头，等价于 gin 的 c.GetHeader()。
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// Header 设置响应头，等价于 gin 的 c.Header()。
func (c *Context) Header(key, value string) {
	c.Writer.Header().Set(key, value)
}

// ClientIP 解析客户端真实 IP（X-Forwarded-For / X-Real-IP / RemoteAddr），
// 等价于 gin 的 c.ClientIP()。
func (c *Context) ClientIP() string {
	if fwd := c.Request.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// responseWriter 包装 http.ResponseWriter，记录状态码与写入字节数，
// 并提供 WriteString / Flush 等 gin 兼容能力。
type responseWriter struct {
	http.ResponseWriter
	status  int
	size    int
	written bool
}

func (w *responseWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
	w.written = true
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// Status 返回已写入的状态码；尚未写入时返回 0（与 gin 行为一致）。
func (w *responseWriter) Status() int {
	if w.written {
		return w.status
	}
	return 0
}

// Size 返回已写入的字节数（未压缩原始大小）。
func (w *responseWriter) Size() int {
	return w.size
}

// WriteString 写入字符串。
func (w *responseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush 透传到底层 Flusher（如需要流式响应）。
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 便于标准库 hijack 等场景。
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// gzipWriter 在需要时对响应体做 gzip 压缩。ContentType 指示为不可压缩时自动绕过。
type gzipWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	bypass      bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	if shouldSkipGzip(g.Header().Get("Content-Type")) {
		g.bypass = true
	} else {
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Del("Content-Length")
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.bypass {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

func (g *gzipWriter) Flush() {
	if !g.bypass {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// shouldSkipGzip 判断某 Content-Type 是否不应再压缩（图片 / 音视频 / 已压缩体）。
func shouldSkipGzip(ct string) bool {
	if ct == "" {
		return false
	}
	switch {
	case strings.HasPrefix(ct, "image/"),
		strings.HasPrefix(ct, "video/"),
		strings.HasPrefix(ct, "audio/"),
		strings.Contains(ct, "gzip"),
		strings.Contains(ct, "zip"),
		strings.Contains(ct, "br"):
		return true
	}
	return false
}

// Router 是顶层路由，组合全局中间件与一个 net/http.ServeMux。
// 它实现了 http.Handler，可直接传给 http.ListenAndServe 或 HTTP/3 服务。
type Router struct {
	mux    *http.ServeMux
	global []HandlerFunc
	gzip   bool
}

// NewRouter 创建一个默认开启 gzip 的路由。
func NewRouter() *Router {
	return &Router{mux: http.NewServeMux(), gzip: true}
}

// Use 注册全局中间件（作用于所有路由），等价于 gin 的 r.Use()。
func (rt *Router) Use(h ...HandlerFunc) {
	rt.global = append(rt.global, h...)
}

// EnableGzip 控制是否对响应启用 gzip（默认开启）。
func (rt *Router) EnableGzip(v bool) {
	rt.gzip = v
}

// Group 创建一个带前缀的路由组，等价于 gin 的 r.Group()。
func (rt *Router) Group(prefix string, h ...HandlerFunc) *RouterGroup {
	return &RouterGroup{rt: rt, prefix: prefix, handlers: h}
}

// GET / POST / PUT / DELETE / PATCH 在根前缀注册路由。
func (rt *Router) GET(path string, h ...HandlerFunc)   { rt.handle(http.MethodGet, path, h...) }
func (rt *Router) POST(path string, h ...HandlerFunc)  { rt.handle(http.MethodPost, path, h...) }
func (rt *Router) PUT(path string, h ...HandlerFunc)    { rt.handle(http.MethodPut, path, h...) }
func (rt *Router) DELETE(path string, h ...HandlerFunc) { rt.handle(http.MethodDelete, path, h...) }
func (rt *Router) PATCH(path string, h ...HandlerFunc)  { rt.handle(http.MethodPatch, path, h...) }

// NoRoute 注册一个兜底处理器（用于 SPA 回退），等价于 gin 的 r.NoRoute()。
// 仅匹配 GET，避免影响 /api 下的未匹配请求（直接 404）。
func (rt *Router) NoRoute(h HandlerFunc) {
	rt.mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		c := rt.newRouteContext(w, r, []HandlerFunc{h})
		c.Next()
	})
}

// ServeHTTP 实现 http.Handler：装配全局中间件链，链尾交给 mux 分发。
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &responseWriter{ResponseWriter: w}
	var gzw *gzipWriter
	if rt.gzip && canGzip(r) {
		gzw = &gzipWriter{ResponseWriter: w, gz: gzip.NewWriter(w)}
		rw = &responseWriter{ResponseWriter: gzw}
	}
	rs := &requestState{rw: rw, keys: map[string]any{}}
	r = r.WithContext(context.WithValue(r.Context(), stateKey, rs))

	c := &Context{Writer: rw, Request: r, state: rs, handlers: append([]HandlerFunc{}, rt.global...), index: -1}
	c.handlers = append(c.handlers, func(c *Context) { rt.mux.ServeHTTP(c.Writer, c.Request) })
	c.Next()

	if gzw != nil && !gzw.bypass {
		_ = gzw.gz.Close()
	}
}

func (rt *Router) handle(method, path string, h ...HandlerFunc) {
	rt.mux.HandleFunc(method+" "+convertPath(path), func(w http.ResponseWriter, r *http.Request) {
		c := rt.newRouteContext(w, r, h)
		c.Next()
	})
}

func (rt *Router) newRouteContext(w http.ResponseWriter, r *http.Request, handlers []HandlerFunc) *Context {
	rw, ok := w.(*responseWriter)
	if !ok {
		rw = &responseWriter{ResponseWriter: w}
	}
	rs, _ := r.Context().Value(stateKey).(*requestState)
	return &Context{Writer: rw, Request: r, state: rs, handlers: handlers, index: -1}
}

func canGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

// RouterGroup 等价于 gin.RouterGroup：带前缀与组级中间件的子路由。
type RouterGroup struct {
	rt       *Router
	prefix   string
	handlers []HandlerFunc
}

// Group 创建子路由组，继承当前组的前缀与中间件，等价于 gin 的 rg.Group()。
func (g *RouterGroup) Group(prefix string, h ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		rt:       g.rt,
		prefix:   g.prefix + prefix,
		handlers: append(append([]HandlerFunc{}, g.handlers...), h...),
	}
}

// Use 注册组级中间件，等价于 gin 的 rg.Use()。
func (g *RouterGroup) Use(h ...HandlerFunc) {
	g.handlers = append(g.handlers, h...)
}

// GET / POST / PUT / DELETE / PATCH 在组前缀下注册路由。
func (g *RouterGroup) GET(path string, h ...HandlerFunc)   { g.handle(http.MethodGet, path, h...) }
func (g *RouterGroup) POST(path string, h ...HandlerFunc)  { g.handle(http.MethodPost, path, h...) }
func (g *RouterGroup) PUT(path string, h ...HandlerFunc)    { g.handle(http.MethodPut, path, h...) }
func (g *RouterGroup) DELETE(path string, h ...HandlerFunc) { g.handle(http.MethodDelete, path, h...) }
func (g *RouterGroup) PATCH(path string, h ...HandlerFunc)  { g.handle(http.MethodPatch, path, h...) }

func (g *RouterGroup) handle(method, path string, h ...HandlerFunc) {
	combined := append(append([]HandlerFunc{}, g.handlers...), h...)
	pattern := method + " " + g.prefix + convertPath(path)
	g.rt.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		c := g.rt.newRouteContext(w, r, combined)
		c.Next()
	})
}

// convertPath 把 gin 风格路径（:name、*name）转换为 Go 1.22 ServeMux 的
// {name} / {name...} 通配语法。
func convertPath(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == ':' || c == '*' {
			j := i + 1
			for j < len(p) && p[j] != '/' {
				j++
			}
			name := p[i+1 : j]
			if c == '*' {
				b.WriteString("{" + name + "...}")
			} else {
				b.WriteString("{" + name + "}")
			}
			i = j - 1
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
