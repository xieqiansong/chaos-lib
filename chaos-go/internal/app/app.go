package app

import (
	_ "chaos-go/internal/domain/cronjob"
	_ "chaos-go/internal/domain/dashboard"
	_ "chaos-go/internal/domain/dict"
	"chaos-go/internal/domain/envvar"
	_ "chaos-go/internal/domain/filelink"
	_ "chaos-go/internal/domain/items"
	_ "chaos-go/internal/domain/polyform"
	_ "chaos-go/internal/domain/portfwd"
	_ "chaos-go/internal/domain/project"
	_ "chaos-go/internal/domain/proxy"
	"chaos-go/internal/domain/quickedit"
	_ "chaos-go/internal/domain/sdk"
	_ "chaos-go/internal/domain/sshconn"
	_ "chaos-go/internal/domain/standarddata"
	_ "chaos-go/internal/domain/systemjob"
	_ "chaos-go/internal/domain/taskplan"
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/config"
	_ "chaos-go/internal/framework/datacache"
	_ "chaos-go/internal/framework/dbmonitor"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/router"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/platform/mqttsync"
	_ "chaos-go/internal/platform/stunpf"
	_ "chaos-go/internal/platform/stunsync"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(webFS fs.FS) error {
	cfg := config.LoadConfig()
	slog.Info("配置加载成功")

	config.InitLog()
	slog.Info("日志初始化完成")

	db := config.GetDB()
	if db == nil {
		slog.Error("数据库连接失败，跳过自动迁移（依赖数据库的接口将报错）")
	}

	regs, err := plugin.Build()
	if err != nil {
		slog.Error("插件依赖校验失败", "err", err)
		return err
	}

	if db != nil {
		migrateModels := append(plugin.ModelsOf(regs), &apilog.ApiLog{})
		if err := config.AutoMigrate(db, migrateModels...); err != nil {
			slog.Error("数据库迁移失败", "err", err)
		} else {
			slog.Info("数据库迁移完成", "models", len(migrateModels))
		}
	}

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	apilog.Start()
	slog.Info("接口访问日志记录器启动完成")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("MQTT 同步启动 panic", "panic", r)
			}
		}()
		mqttsync.Start()
	}()

	quickedit.EnvReadContent = envvar.ReadVirtualContent
	quickedit.EnvWriteContent = envvar.WriteVirtualContent

	if err := plugin.StartAll(regs); err != nil {
		slog.Error("部分插件启动失败，继续以降级模式运行", "err", err)
	}

	scheduler.StartBuiltin(plugin.TasksOf(regs))
	slog.Info("内置任务启动完成")

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	srv := &http.Server{Addr: cfg.Server.GetAddress(), Handler: router.SetupRouter(webFS)}

	sigCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sigCtx.Done():
		slog.Info("收到退出信号，开始优雅退出")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP 服务关闭超时", "err", err)
	}
	if err := plugin.StopAll(regs); err != nil {
		slog.Warn("部分插件停止失败", "err", err)
	}
	slog.Info("已退出")
	return nil
}

func setupPprof(cfg *config.PprofConfig) {
	if !cfg.Enabled {
		slog.Info("Pprof 已禁用")
		return
	}

	go func() {
		slog.Info("Pprof 启动", "addr", cfg.GetAddress())
		if err := http.ListenAndServe(cfg.GetAddress(), nil); err != nil {
			slog.Error("Pprof 启动失败", "err", err)
		}
	}()
}
