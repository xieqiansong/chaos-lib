package main

import (
	"chaos-lib/config"
	"chaos-lib/routes"
	"chaos-lib/tasks"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	_ "net/http/pprof"
)

//go:embed web
var webFS embed.FS

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

func setupPprof(cfg *config.PprofConfig) {
	if !cfg.Enabled {
		slog.Info("Pprof 已禁用")
		return
	}

	go func() {
		slog.Info("Pprof 启动", "addr", cfg.GetAddress())
		err := http.ListenAndServe(cfg.GetAddress(), nil)
		if err != nil {
			slog.Error("Pprof 启动失败", "err", err)
		}
	}()
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

	cfg := config.LoadConfig()
	slog.Info("配置加载成功")

	config.InitLog()
	slog.Info("日志初始化完成")

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	tasks.Start()
	slog.Info("后台任务启动完成")

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress(), "env", cfg.Environment)
	r := routes.SetupRouter(webFS)
	go r.Run(cfg.Server.GetAddress())

	slog.Info("HTTP 服务启动完成")

	select {}
}
