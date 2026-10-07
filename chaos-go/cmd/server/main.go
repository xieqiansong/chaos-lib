package main

import (
	"chaos-go/internal/app"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	_ "net/http/pprof"
)

// resolveUIFS 返回前端静态资源：优先 CHAOS_UI_DIR 环境变量，否则可执行文件同目录 ./ui。
func resolveUIFS() fs.FS {
	if dir := os.Getenv("CHAOS_UI_DIR"); dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			slog.Info("前端 UI 目录（环境变量覆盖）", "dir", dir)
			return os.DirFS(dir)
		}
		slog.Warn("CHAOS_UI_DIR 指定目录不存在，回退默认路径", "dir", dir)
	}
	execPath, err := os.Executable()
	if err != nil {
		slog.Error("无法定位可执行文件", "err", err)
		return os.DirFS(".")
	}
	uiDir := filepath.Join(filepath.Dir(execPath), "ui")
	if st, err := os.Stat(uiDir); err != nil || !st.IsDir() {
		slog.Error("前端 UI 目录不存在，界面将不可用（前端产物应放在 exe 同目录 ./ui）", "dir", uiDir)
	}
	return os.DirFS(uiDir)
}

func initEarlyLog() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)
	logPath := filepath.Join(execDir, "logs", "startup.log")

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
}

func main() {
	initEarlyLog()
	slog.Info("程序启动", "time", time.Now().Format("2006-01-02 15:04:05"))

	defer func() {
		if r := recover(); r != nil {
			slog.Error("程序崩溃", "panic", r, "stack", string(debug.Stack()))
			execPath, _ := os.Executable()
			execDir := filepath.Dir(execPath)
			crashPath := filepath.Join(execDir, "crash.log")
			f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				defer f.Close()
				slog.New(slog.NewTextHandler(f, nil)).Error("程序崩溃", "panic", r, "stack", string(debug.Stack()))
			}
		}
	}()

	if err := app.Run(resolveUIFS()); err != nil {
		slog.Error("HTTP 服务退出", "err", err)
	}
}
