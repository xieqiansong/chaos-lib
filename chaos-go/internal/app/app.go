// Package app 是应用的装配根（composition root）：
// 负责按正确顺序加载配置、初始化依赖、执行迁移、启动后台任务并拉起 HTTP Server。
// cmd/server/main.go 仅做进程级引导（早期日志、崩溃恢复、UI 路径解析），其余统一在此编排。
package app

import (
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/config"
	"chaos-go/internal/domain/cronjob"
	"chaos-go/internal/framework/datacache"
	_ "chaos-go/internal/framework/dbmonitor"
	"chaos-go/internal/domain/envvar"
	"chaos-go/internal/domain/filelink"
	"chaos-go/internal/platform/mqttsync"
	"chaos-go/internal/domain/note"
	"chaos-go/internal/domain/portfwd"
	"chaos-go/internal/domain/project"
	"chaos-go/internal/domain/proxy"
	"chaos-go/internal/domain/quickedit"
	"chaos-go/internal/framework/router"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/domain/sdk"
	"chaos-go/internal/domain/standarddata"
	_ "chaos-go/internal/domain/systemjob"
	"chaos-go/internal/domain/taskplan"
	"io/fs"
	"log/slog"
	"net/http"

	"crypto/tls"
	"fmt"

	"github.com/quic-go/quic-go/http3"
	security "chaos-go/internal/framework/security"
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
		&note.Note{},
		&note.NoteTag{},
		&note.NoteTagRel{},
		&note.NoteLink{},
		&apilog.ApiLog{},
	); err != nil {
		slog.Error("数据库迁移失败", "err", err)
	}

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	// 接口访问日志：启动异步批量落库消费者（此前由包 init 起 goroutine，现改为显式启动）
	apilog.Start()

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

	cronjob.Start()

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	r := router.SetupRouter(webFS)

	if cfg.Server.EnableHTTP3 {
		return runHTTP3(r, &cfg.Server)
	}
	return r.Run(cfg.Server.GetAddress())
}

// runHTTP3 在启用 HTTP/3 时同时拉起 HTTPS(TCP) 与 HTTP/3(UDP/QUIC) 两个服务，
// 复用同一个 handler；并通过 Alt-Svc 响应头告知浏览器可升级到 h3（前端代码无需改动）。
func runHTTP3(handler http.Handler, srv *config.ServerConfig) error {
	http3Srv := &http3.Server{
		Addr: srv.GetHTTP3Address(),
		Port: srv.GetHTTP3Port(),
		Handler: handler,
	}

	base, err := loadTLSConfig(srv)
	if err != nil {
		return fmt.Errorf("加载 TLS 配置失败: %w", err)
	}
	// TCP 侧优先协商 HTTP/2，UDP(QUIC) 侧由 ConfigureTLSConfig 设为 h3。
	tlsTCP := base.Clone()
	tlsTCP.NextProtos = []string{"h2", "http/1.1"}
	http3Srv.TLSConfig = http3.ConfigureTLSConfig(base.Clone())

	// Alt-Svc：在 TCP(HTTPS) 响应里告知浏览器本服务支持 HTTP/3，浏览器自动协商升级（前端无感）。
	withAltSvc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = http3Srv.SetQUICHeaders(w.Header())
		handler.ServeHTTP(w, req)
	})
	tcpSrv := &http.Server{
		Addr:      srv.GetAddress(),
		Handler:   withAltSvc,
		TLSConfig: tlsTCP,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("HTTP/3(QUIC/UDP) 服务启动", "addr", srv.GetHTTP3Address())
		errCh <- http3Srv.ListenAndServe()
	}()
	go func() {
		ln, lerr := tls.Listen("tcp", srv.GetAddress(), tlsTCP)
		if lerr != nil {
			errCh <- lerr
			return
		}
		slog.Info("HTTPS(TCP) 服务启动", "addr", srv.GetAddress())
		errCh <- tcpSrv.Serve(ln)
	}()

	slog.Warn("已启用 HTTPS + HTTP/3（自签名证书），浏览器会提示不安全，需手动信任；生产环境请改用受信任证书")
	return <-errCh
}

// loadTLSConfig 加载 TLS 配置：配置了证书文件则直接加载，否则生成自签名证书并落盘（方案 A）。
func loadTLSConfig(srv *config.ServerConfig) (*tls.Config, error) {
	if srv.TLSCertFile != "" && srv.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(srv.TLSCertFile, srv.TLSKeyFile)
		if err != nil {
			return nil, err
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	self, err := security.GenerateSelfSigned()
	if err != nil {
		return nil, err
	}
	certPath, keyPath, err := self.Save("certs")
	if err != nil {
		return nil, err
	}
	slog.Info("已生成自签名证书，请将 certs/chaos.crt 导入系统/浏览器信任",
		"cert", certPath, "key", keyPath)
	return &tls.Config{Certificates: []tls.Certificate{self.Certificate}}, nil
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
