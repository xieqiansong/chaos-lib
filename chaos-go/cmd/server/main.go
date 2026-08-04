package main

import (
	"chaos-go/config"
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	_ "chaos-go/internal/notify"
	"chaos-go/internal/portfwd"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	"chaos-go/internal/taskplan"
	"chaos-go/routes"
	"chaos-go/scheduler"
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

	// 数据库连接 & 自动迁移
	db := config.GetDB()
	slog.Info("数据库连接成功，执行自动迁移")
	if err := config.AutoMigrate(db,
		&taskplan.TaskPlan{},
		&taskplan.Task{},
		&project.ProjectGroup{},
		&project.Project{},
		&proxy.BrowserHistory{},
		&proxy.BrowserHistoryVisit{},
		&portfwd.PortForwarding{},
		&filelink.FileLink{},
		&quickedit.QuickEditFile{},
		&quickedit.QuickEditSnapshot{},
		&proxy.SdkSource{},
	); err != nil {
		slog.Error("数据库迁移失败", "err", err)
	}

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	// 注入 quickedit env 回调（避免循环依赖）
	quickedit.EnvReadContent = envvar.ReadVirtualContent
	quickedit.EnvWriteContent = envvar.WriteVirtualContent

	scheduler.Start()
	slog.Info("后台任务启动完成")

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress(), "env", cfg.Environment)
	r := routes.SetupRouter(webFS)
	go r.Run(cfg.Server.GetAddress())

	slog.Info("HTTP 服务启动完成")

	select {}
}