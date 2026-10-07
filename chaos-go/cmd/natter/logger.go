package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	colorGrey       = "\033[90;20m"
	colorYellowBold = "\033[33;1m"
	colorRedBold    = "\033[31;1m"
	colorReset      = "\033[0m"
)

func levelRep(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "D"
	case l < slog.LevelWarn:
		return "I"
	case l < slog.LevelError:
		return "W"
	default:
		return "E"
	}
}

// natterHandler 自定义 slog.Handler，格式沿用 Python 版。
type natterHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Level
	color bool
}

func (h *natterHandler) Enabled(_ context.Context, lvl slog.Level) bool { return lvl >= h.level }

func (h *natterHandler) Handle(_ context.Context, r slog.Record) error {
	prefix, suffix := "", ""
	if h.color {
		switch {
		case r.Level < slog.LevelInfo:
			prefix, suffix = colorGrey, colorReset
		case r.Level >= slog.LevelError:
			prefix, suffix = colorRedBold, colorReset
		case r.Level >= slog.LevelWarn:
			prefix, suffix = colorYellowBold, colorReset
		}
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(r.Time.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, " [%s] %s", levelRep(r.Level), r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" ")
		b.WriteString(a.Key)
		b.WriteString("=")
		b.WriteString(a.Value.String())
		return true
	})
	b.WriteString(suffix)
	b.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *natterHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *natterHandler) WithGroup(string) slog.Handler      { return h }

// colorEnabled 沿用 Python 版的判断：stderr 是终端且 TERM 含 256color。
func colorEnabled() bool {
	if !strings.Contains(os.Getenv("TERM"), "256color") {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// setupLogger 安装 natter 风格日志；verbose 时降到 debug 级别。
func setupLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(&natterHandler{
		mu:    &sync.Mutex{},
		w:     os.Stderr,
		level: level,
		color: colorEnabled(),
	}))
}

// sleepCtx 等待指定时长，期间 ctx 取消则立即返回。
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
