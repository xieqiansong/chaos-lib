package tools

import (
	"io"
	"log"
	"log/slog"
	"os"
)

// Must 万能断言函数，利用泛型一劳永逸：err 非 nil 时直接退出进程。
func Must[T any](v T, err error) T {
	if err != nil {
		slog.Error("err", err)
		os.Exit(1)
	}
	return v
}

// Must0 当不需要返回值时，仅检查错误，err 非 nil 时直接 fatal。
func Must0(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// CloseQuietly 静默关闭实现 io.Closer 的资源（如 *sql.DB / *sql.Rows /
// http.Response.Body），仅把 Close 返回的错误记入日志，不中断调用流程。
// 自带 nil 保护，可直接用于 defer：
//
//	defer tools.CloseQuietly(db)
func CloseQuietly(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		slog.Error("关闭资源失败", "err", err)
	}
}
