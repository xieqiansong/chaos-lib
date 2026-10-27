package tools

import (
	"io"
	"log"
	"log/slog"
	"os"
)

func Must[T any](v T, err error) T {
	if err != nil {
		slog.Error("err", "err", err)
		os.Exit(1)
	}
	return v
}

func Must0(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CloseQuietly(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		slog.Error("关闭资源失败", "err", err)
	}
}
