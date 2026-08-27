package tools

import (
	"log"
	"log/slog"
	"os"
)

// 万能 Must 函数，利用泛型，一劳永逸
func Must[T any](v T, err error) T {
	if err != nil {
		slog.Error("err", err)
		os.Exit(1)
	}
	return v
}

// 如果不需要返回值，只检查错误
func Must0(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
