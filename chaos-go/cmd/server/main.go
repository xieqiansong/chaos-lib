package main

import (
	"chaos-go/config"
	"chaos-go/internal/envvar"
	"chaos-go/internal/filelink"
	mqttsync "chaos-go/internal/mqttsync"
	_ "chaos-go/internal/notify"
	"chaos-go/internal/portfwd"
	"chaos-go/internal/project"
	"chaos-go/internal/proxy"
	"chaos-go/internal/quickedit"
	_ "chaos-go/internal/stunsync"
	_ "chaos-go/internal/stunpf"
	"chaos-go/internal/taskplan"
	"chaos-go/routes"
	"chaos-go/scheduler"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	_ "net/http/pprof"
)

// resolveUIFS 返回前端静态资源文件系统：
// 优先 CHAOS_UI_DIR 环境变量（便于开发时直指 chaos-ui/dist），
// 否则用可执行文件同目录的 ./ui（部署产物，不内嵌）。
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
		&portfwd.SshConnection{},
		&filelink.FileLink{},
		&quickedit.QuickEditFile{},
		&quickedit.QuickEditSnapshot{},
		&proxy.SdkSource{},
		&mqttsync.MqttSyncMessage{},
		&mqttsync.MqttSyncNode{},
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

	// 端口转发自愈：周期性重连"期望运行但实际未运行"的规则（覆盖重启恢复失败、隧道意外消亡等）
	scheduler.Register("端口转发自愈", 5*time.Minute, portfwd.SelfHealForwards)

	scheduler.Start()
	slog.Info("后台任务启动完成")

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	r := routes.SetupRouter(resolveUIFS())
	go r.Run(cfg.Server.GetAddress())

	slog.Info("HTTP 服务启动完成")

	select {}
}
