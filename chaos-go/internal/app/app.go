// Package app 是应用的装配根（composition root）：
// 负责按正确顺序加载配置、初始化依赖、执行迁移、启动后台任务并拉起 HTTP Server。
// cmd/server/main.go 仅做进程级引导（早期日志、崩溃恢复、UI 路径解析），其余统一在此编排。
package app

import (
	_ "chaos-go/internal/domain/cronjob"
	"chaos-go/internal/domain/envvar"
	_ "chaos-go/internal/domain/filelink"
	itemsdomain "chaos-go/internal/domain/items"
	_ "chaos-go/internal/domain/note"
	_ "chaos-go/internal/domain/portfwd"
	_ "chaos-go/internal/domain/project"
	_ "chaos-go/internal/domain/proxy"
	"chaos-go/internal/domain/quickedit"
	_ "chaos-go/internal/domain/sdk"
	_ "chaos-go/internal/domain/standarddata"
	_ "chaos-go/internal/domain/systemjob"
	_ "chaos-go/internal/domain/taskplan" // 副作用导入：触发其 init() 登记插件
	_ "chaos-go/internal/domain/dashboard"
	_ "chaos-go/internal/domain/sshconn"
	_ "chaos-go/internal/domain/polyform"
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/config"
	_ "chaos-go/internal/framework/datacache"
	_ "chaos-go/internal/framework/dbmonitor"
	"chaos-go/internal/framework/plugin"
	"chaos-go/internal/framework/router"
	"chaos-go/internal/framework/scheduler"
	_ "chaos-go/internal/platform/mqttsync"
	"io/fs"
	"log/slog"
	"net/http"

	"crypto/tls"
	"fmt"
	"path/filepath"

	security "chaos-go/internal/framework/security"

	"github.com/quic-go/quic-go/http3"
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

	// 插件依赖校验：缺失依赖 / 循环依赖在此 fail-fast。
	// 已登记为插件的模块：proxy / taskplan / project / note / portfwd / envvar / filelink / sdk /
	// standarddata / quickedit / systemjob / cronjob / mqttsync / dbmonitor / datacache /
	// dashboard / sshconn / polyform（模型与 Start 在各自插件内声明）；
	// 其余存量项逐步下沉。
	regs, err := plugin.Build()
	if err != nil {
		slog.Error("插件依赖校验失败", "err", err)
		return err
	}

	slog.Info("数据库连接成功，执行自动迁移")
	migrateModels := plugin.ModelsOf(regs) // 插件声明的模型（含 dbmonitor 无模型、datacache 的 DataCache 等）
	migrateModels = append(migrateModels,
		&apilog.ApiLog{},
	)
	if err := config.AutoMigrate(db, migrateModels...); err != nil {
		slog.Error("数据库迁移失败", "err", err)
	}

	// 个人物品模块：初始化默认分类 / 位置字典（幂等）
	if err := itemsdomain.Seed(); err != nil {
		slog.Error("个人物品字典初始化失败", "err", err)
	}

	setupPprof(&cfg.Pprof)
	slog.Info("Pprof 设置完成")

	// 接口访问日志：启动异步批量落库消费者（此前由包 init 起 goroutine，现改为显式启动）
	apilog.Start()
	slog.Info("接口访问日志记录器启动完成")

	// 多节点 MQTT 同步：未启用或 broker 不可达时不阻塞启动
	// mqttsync.Start()
	slog.Info("多节点 MQTT 同步启动完成")

	// 注入 quickedit env 回调（避免循环依赖）
	quickedit.EnvReadContent = envvar.ReadVirtualContent
	quickedit.EnvWriteContent = envvar.WriteVirtualContent

	scheduler.Start()
	slog.Info("后台任务启动完成")

	// 插件生命周期启动（按依赖拓扑序）。taskplan / project / note / filelink / sdk /
	// standarddata / envvar / quickedit / mqttsync / systemjob / dbmonitor / datacache /
	// dashboard / sshconn / polyform 无 Start；
	// portfwd 的 Start 触发隧道恢复，cronjob 的 Start 触发 cron 调度运行时。
	if err := plugin.StartAll(regs); err != nil {
		slog.Error("插件启动失败", "err", err)
		return err
	}

	slog.Info("启动 HTTP 服务", "addr", cfg.Server.GetAddress())
	r := router.SetupRouter(webFS)

	if cfg.Server.EnableHTTP3 {
		return runHTTP3(r, &cfg.Server)
	}
	return http.ListenAndServe(cfg.Server.GetAddress(), r)
}

// runHTTP3 在启用 HTTP/3 时同时拉起 HTTPS(TCP) 与 HTTP/3(UDP/QUIC) 两个服务，
// 复用同一个 handler；并通过 Alt-Svc 响应头告知浏览器可升级到 h3（前端代码无需改动）。
func runHTTP3(handler http.Handler, srv *config.ServerConfig) error {
	base, err := loadTLSConfig(srv)
	if err != nil {
		return fmt.Errorf("加载 TLS 配置失败: %w", err)
	}

	http3Srv := &http3.Server{
		Addr:      srv.GetHTTP3Address(),
		Port:      srv.GetHTTP3Port(),
		Handler:   handler,
		TLSConfig: http3.ConfigureTLSConfig(base.Clone()),
	}

	// TCP 侧优先协商 HTTP/2，UDP(QUIC) 侧由 ConfigureTLSConfig 设为 h3。
	tlsTCP := base.Clone()
	tlsTCP.NextProtos = []string{"h2", "http/1.1"}

	// Alt-Svc：在 HTTPS 响应里告知浏览器本服务支持 HTTP/3，浏览器自动协商升级（前端无感）。
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

	// 方案 A：自签名证书持久化——已存在且可加载则复用，避免每次启动都换证书导致需重新信任。
	certPath := filepath.Join("certs", "chaos.crt")
	keyPath := filepath.Join("certs", "chaos.key")
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		slog.Info("复用已存在的自签名证书（删除 certs/ 目录可强制重新生成）", "cert", certPath)
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	self, err := security.GenerateSelfSigned()
	if err != nil {
		return nil, err
	}
	if _, _, err := self.Save("certs"); err != nil {
		return nil, err
	}
	slog.Info("已生成自签名证书，请将 certs/chaos.crt 导入系统/浏览器信任（仅首次需要）",
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
