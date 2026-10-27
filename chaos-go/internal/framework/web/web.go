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

type HandlerFunc func(c *Context)

type contextKey string

const stateKey contextKey = "web.requestState"

type requestState struct {
	rw   *responseWriter
	keys map[string]any
	errs []error
}

type Context struct {
	Writer *responseWriter

	Request *http.Request

	state    *requestState
	handlers []HandlerFunc
	index    int
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *Context) Abort() {
	c.index = len(c.handlers)
}

func (c *Context) Set(key string, value any) {
	c.state.keys[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	v, ok := c.state.keys[key]
	return v, ok
}

func (c *Context) MustGet(key string) any {
	v, ok := c.state.keys[key]
	if !ok {
		panic(fmt.Sprintf("web: context missing key %q", key))
	}
	return v
}

func (c *Context) Error(err error) {
	if err != nil {
		c.state.errs = append(c.state.errs, err)
	}
}

func (c *Context) Errors() []error {
	return c.state.errs
}

func (c *Context) JSON(code int, obj any) {
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	_ = json.NewEncoder(c.Writer).Encode(obj)
}

func (c *Context) Data(code int, contentType string, data []byte) {
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	c.Writer.WriteHeader(code)
	_, _ = c.Writer.Write(data)
}

func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

func (c *Context) String(code int, format string, values ...any) {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	fmt.Fprintf(c.Writer, format, values...)
}

func (c *Context) Param(name string) string {
	return c.Request.PathValue(name)
}

func (c *Context) Query(name string) string {
	return c.Request.URL.Query().Get(name)
}

func (c *Context) DefaultQuery(name, def string) string {
	if v := c.Request.URL.Query().Get(name); v != "" {
		return v
	}
	return def
}

func (c *Context) PostForm(name string) string {
	_ = c.Request.ParseForm()
	return c.Request.PostFormValue(name)
}

func (c *Context) ShouldBindJSON(dst any) error {
	return json.NewDecoder(c.Request.Body).Decode(dst)
}

func (c *Context) BindJSON(dst any) error {
	return c.ShouldBindJSON(dst)
}

func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Context) Header(key, value string) {
	c.Writer.Header().Set(key, value)
}

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

func (w *responseWriter) Status() int {
	if w.written {
		return w.status
	}
	return 0
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

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

type Router struct {
	mux    *http.ServeMux
	global []HandlerFunc
	gzip   bool
}

func NewRouter() *Router {
	return &Router{mux: http.NewServeMux(), gzip: true}
}

func (rt *Router) Use(h ...HandlerFunc) {
	rt.global = append(rt.global, h...)
}

func (rt *Router) EnableGzip(v bool) {
	rt.gzip = v
}

func (rt *Router) Group(prefix string, h ...HandlerFunc) *RouterGroup {
	return &RouterGroup{rt: rt, prefix: prefix, handlers: h}
}

func (rt *Router) GET(path string, h ...HandlerFunc)    { rt.handle(http.MethodGet, path, h...) }
func (rt *Router) POST(path string, h ...HandlerFunc)   { rt.handle(http.MethodPost, path, h...) }
func (rt *Router) PUT(path string, h ...HandlerFunc)    { rt.handle(http.MethodPut, path, h...) }
func (rt *Router) DELETE(path string, h ...HandlerFunc) { rt.handle(http.MethodDelete, path, h...) }
func (rt *Router) PATCH(path string, h ...HandlerFunc)  { rt.handle(http.MethodPatch, path, h...) }

func (rt *Router) NoRoute(h HandlerFunc) {
	rt.mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		c := rt.newRouteContext(w, r, []HandlerFunc{h})
		c.Next()
	})
}

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

type RouterGroup struct {
	rt       *Router
	prefix   string
	handlers []HandlerFunc
}

func (g *RouterGroup) Group(prefix string, h ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		rt:       g.rt,
		prefix:   g.prefix + prefix,
		handlers: append(append([]HandlerFunc{}, g.handlers...), h...),
	}
}

func (g *RouterGroup) Use(h ...HandlerFunc) {
	g.handlers = append(g.handlers, h...)
}

func (g *RouterGroup) GET(path string, h ...HandlerFunc)    { g.handle(http.MethodGet, path, h...) }
func (g *RouterGroup) POST(path string, h ...HandlerFunc)   { g.handle(http.MethodPost, path, h...) }
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
