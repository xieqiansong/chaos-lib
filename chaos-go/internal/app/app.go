// Package app 是应用的装配根（composition root）：
// 负责按正确顺序加载配置、初始化依赖、执行迁移、启动后台任务并拉起 HTTP Server。
// cmd/server/main.go 仅做进程级引导（早期日志、崩溃恢复、UI 路径解析），其余统一在此编排。
package app

import (
	_ "chaos-go/internal/domain/cronjob"
	_ "chaos-go/internal/domain/dashboard"
	_ "chaos-go/internal/domain/dict"
	"chaos-go/internal/domain/envvar"
	_ "chaos-go/internal/domain/filelink"
	_ "chaos-go/internal/domain/items" // 副作用导入：触发其 init() 登记 items 插件（含 items/item-records/item-assemblies 路由）
	_ "chaos-go/internal/domain/polyform"
	_ "chaos-go/internal/domain/portfwd"
	_ "chaos-go/internal/domain/project"
	_ "chaos-go/internal/domain/proxy"
	"chaos-go/internal/domain/quickedit"
	_ "chaos-go/internal/domain/sdk"
	_ "chaos-go/internal/domain/sshconn"
	_ "chaos-go/internal/domain/standarddata"
	_ "chaos-go/internal/domain/systemjob"
	_ "chaos-go/internal/domain/taskplan" // 副作用导入：触发其 init() 登记插件
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/database"
	_ "chaos-go/internal/framework/datacache"
	_ "chaos-go/internal/framework/dbmonitor"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/router"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/platform/mqttsync"
	_ "chaos-go/internal/platform/stunpf"
	_ "chaos-go/internal/platform/stunsync"
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

	// 插件依赖校验：缺失依赖 / 循环依赖在此 fail-fast。
	// 已登记为插件的模块：proxy / taskplan / project / portfwd / envvar / filelink / sdk /
	// standarddata / dict / quickedit / systemjob / cronjob / mqttsync / stunpf / stunsync /
	// dbmonitor / datacache / database / dashboard / sshconn / polyform。
	regs, err := plugin.Build()
	if err != nil {
		slog.Error("插件依赖校验失败", "err", err)
		return err
	}

	// 数据库连接与自动迁移由 database 插件负责：连接失败只影响依赖数据库的能力，
	// 不阻断进程启动；依赖数据库初始化的插件自行通过 database.OnReady 补做初始化。
	database.SetModels(append(plugin.ModelsOf(regs), &apilog.ApiLog{}))

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	// 接口访问日志：启动异步批量落库消费者（此前由包 init 起 goroutine，现改为显式启动）
	apilog.Start()
	slog.Info("接口访问日志记录器启动完成")

	// 多节点 MQTT 同步：未启用、broker 不可达、数据库不可用均不阻塞启动。
	// Start 内部对 broker 是「连接重试 + 阻塞等待」的语义，故放到后台 goroutine，
	// 由 paho 的 ConnectRetry 持续重连，HTTP 服务不受影响。
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("MQTT 同步启动 panic", "panic", r)
			}
		}()
		mqttsync.Start()
	}()
	slog.Info("多节点 MQTT 同步转入后台启动")

	// 注入 quickedit env 回调（避免循环依赖）
	quickedit.EnvReadContent = envvar.ReadVirtualContent
	quickedit.EnvWriteContent = envvar.WriteVirtualContent

	// 插件生命周期启动（按依赖拓扑序）。单个插件失败只记录错误，不阻断其它插件与 HTTP 服务；
	// portfwd 的 Start 触发隧道恢复，cronjob 的 Start 触发用户任务调度。
	if err := plugin.StartAll(regs); err != nil {
		slog.Error("部分插件启动失败，继续以降级模式运行", "err", err)
	}

	// 插件内置任务（代码声明，不落 cron_jobs 表）由框架调度器统一运行。
	scheduler.StartBuiltin(plugin.TasksOf(regs))
	slog.Info("内置任务启动完成")

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	r := router.SetupRouter(webFS)

	return http.ListenAndServe(cfg.Server.GetAddress(), r)
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
