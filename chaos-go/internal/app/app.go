// Package app 是应用的装配根（composition root）：
// 负责按正确顺序加载配置、初始化依赖、执行迁移、启动后台任务并拉起 HTTP Server。
// cmd/server/main.go 仅做进程级引导（早期日志、崩溃恢复、UI 路径解析），其余统一在此编排。
package app

import (
	"chaos-go/internal/apilog"
	"chaos-go/internal/config"
	"chaos-go/internal/cronjob"
	"chaos-go/internal/datacache"
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	"chaos-go/internal/mqttsync"
	"chaos-go/internal/portfwd"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	"chaos-go/internal/router"
	"chaos-go/internal/scheduler"
	"chaos-go/internal/sdk"
	"chaos-go/internal/standarddata"
	"chaos-go/internal/taskplan"
	"io/fs"
	"log/slog"
	"net/http"
)

// Run 装配并启动 chaos-go 应用，阻塞至 HTTP Server 退出。
// webFS 为前端静态资源文件系统（由调用方解析后传入）。
func Run(webFS fs.FS) error {
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
		&proxy.Bookmark{},
		&portfwd.PortForwarding{},
		&portfwd.SshConnection{},
		&filelink.FileLink{},
		&quickedit.QuickEditFile{},
		&quickedit.QuickEditSnapshot{},
		&sdk.SdkSource{},
		&mqttsync.MqttSyncMessage{},
		&mqttsync.MqttSyncNode{},
		&cronjob.CronJob{},
		&cronjob.CronJobRun{},
		&standarddata.StandardData{},
		&datacache.DataCache{},
		&apilog.ApiLog{},
	); err != nil {
		slog.Error("数据库迁移失败", "err", err)
	}

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	// 多节点 MQTT 同步：未启用或 broker 不可达时不阻塞启动
	mqttsync.Start()

	// 进程重启后内存中没有任何隧道：按库里"期望运行"（status=true）的规则重建转发，
	// 实现断线/重启自动重连，而非旧逻辑那样把所有状态清零。
	portfwd.RecoverForwardsOnBoot()

	// 注入 quickedit env 回调（避免循环依赖）
	quickedit.EnvReadContent = envvar.ReadVirtualContent
	quickedit.EnvWriteContent = envvar.WriteVirtualContent

	scheduler.Start()
	slog.Info("后台任务启动完成")

	// 定时任务模块：首次运行时写入由原内置周期任务转换而来的默认任务，
	// 随后启动基于 robfig/cron 的调度器，按 cron 表达式触发各类动作。
	cronjob.SeedDefaults()
	cronjob.Start()

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	r := router.SetupRouter(webFS)
	return r.Run(cfg.Server.GetAddress())
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
