// Natter 的 Go 实现：在全锥 NAT 后打洞，把本地端口暴露到公网。
//
// 对应 Python 版 chaos-python/util/natter.py（Natter v2.2.1, MikeWang000000）。
// 思路与 Python 版一致：
//  1. 用 STUN 从「指定的本地 IP:端口」拿到 NAT 映射后的公网地址；
//  2. 用同一个本地端口建立保活连接，维持映射；
//  3. 把本地端口的入站流量转发到目标地址（iptables / nftables / socket / socat / gost 等）；
//  4. 可选启用 UPnP，在路由器上再加一条端口映射。
//
// 用法（在 chaos-go 模块根目录）：
//
//	go run ./cmd/natter                           # 测试模式，直接起一个应答服务
//	go run ./cmd/natter -t 192.168.1.10 -p 8080
//	go run ./cmd/natter -m iptables -t 192.168.1.10 -p 8080
//	go run ./cmd/natter -u -k 15 -s stun.miwifi.com -U
//
// 与 Python 版的差异：
//   - 日志走自定义 slog.Handler，格式仍是 `[D/I/W/E]` + 时间戳；
//   - Python 的 natterutils.reuse_port（C 扩展）由 socket 的 SO_REUSEPORT 直接替代；
//   - `--check` 需要同目录下的 natter-check.py，由 python 解释器执行。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// natterVersion 与 Python 版保持一致。
const natterVersion = "2.2.1"

// 退出与重试的信号错误，对应 Python 的 NatterExitException / NatterRetryException。
var (
	errNatterExit  = errors.New("natter exit")
	errNatterRetry = errors.New("natter retry")
)

// exitManager 收集退出时需要执行的清理动作，对应 Python 的 NatterExit。
type exitManager struct {
	fns []func()
}

var atExit = &exitManager{}

// Set 注册退出时的清理动作。
func (e *exitManager) Set(fn func()) { e.fns = append(e.fns, fn) }

// Run 逆序执行并清空所有清理动作，可重复调用。
func (e *exitManager) Run() {
	for i := len(e.fns) - 1; i >= 0; i-- {
		e.fns[i]()
	}
	e.fns = nil
}

// stringList 支持 `-s` 这类可重复出现的参数。
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, " ") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// options 汇总命令行参数。
type options struct {
	verbose         bool
	exitWhenChanged bool
	udpMode         bool
	upnpEnabled     bool
	keepRetry       bool
	interval        int
	stunList        stringList
	keepaliveSrv    string
	notifySh        string
	bindIP          string
	bindPort        int
	method          string
	toIP            string
	toPort          int
}

// defaultStunServers 是 TCP 模式下的默认 STUN 服务器列表。
var defaultStunServers = []string{
	"fwa.lifesizecloud.com",
	"global.turn.twilio.com",
	"turn.cloudflare.com",
	"stun.nextcloud.com",
	"stun.freeswitch.org",
	"stun.voip.blackberry.com",
	"stun.sipnet.com",
	"stun.radiojar.com",
	"stun.sonetel.com",
	"stun.telnyx.com",
}

// defaultUDPStunServers 是 UDP 模式下额外前置的 STUN 服务器列表。
var defaultUDPStunServers = []string{
	"stun.miwifi.com",
	"stun.chat.bilibili.com",
	"stun.hitv.com",
	"stun.cdnbye.com",
	"stun.douyucdn.cn:18000",
}

// printUsage 输出帮助信息。
func printUsage() {
	fmt.Fprint(os.Stderr, `Natter (Go) - Expose your port behind full-cone NAT to the Internet.

用法:
    natter [选项]

选项:
    --version, -V          显示版本并退出
    --help                 显示帮助并退出
    --check                运行 natter-check.py 后退出
    -v                     详细模式，打印调试日志
    -q                     映射地址变化时直接退出
    -u                     UDP 模式
    -U                     启用 UPnP/IGD 发现
    -k <interval>          保活间隔秒数（默认 15）
    -s <address>           STUN 服务器地址，可重复指定
    -h <address>           保活服务器地址
    -e <path>              映射地址变化时调用的脚本

绑定选项:
    -i <interface>         绑定的网卡名或 IP（默认 0.0.0.0）
    -b <port>              绑定的端口号（默认由内核分配）

转发选项:
    -m <method>            转发方法：none / test / iptables / sudo-iptables /
                           iptables-snat / sudo-iptables-snat / nftables /
                           sudo-nftables / nftables-snat / sudo-nftables-snat /
                           socat / gost / socket
    -t <address>           转发目标 IP（默认 0.0.0.0）
    -p <port>              转发目标端口（默认取映射后的公网端口）
    -r                     目标端口未开放时持续重试
`)
}

// runNatterCheck 执行同目录下的 natter-check.py。
func runNatterCheck() error {
	const script = "natter-check.py"
	if _, err := os.Stat(script); err != nil {
		return errors.New("natter-check.py is missing")
	}
	abs, err := filepath.Abs(script)
	if err != nil {
		return err
	}
	py := "python"
	if _, err := exec.LookPath(py); err != nil {
		py = "python3"
	}
	cmd := exec.Command(py, abs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// parseFlags 解析命令行参数。
func parseFlags() (*options, error) {
	opt := &options{}
	fs := flag.NewFlagSet("natter", flag.ContinueOnError)
	fs.Usage = printUsage

	var showVersion, showHelp, check bool
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "V", false, "")
	fs.BoolVar(&showHelp, "help", false, "")
	fs.BoolVar(&check, "check", false, "")
	fs.BoolVar(&opt.verbose, "v", false, "")
	fs.BoolVar(&opt.exitWhenChanged, "q", false, "")
	fs.BoolVar(&opt.udpMode, "u", false, "")
	fs.BoolVar(&opt.upnpEnabled, "U", false, "")
	fs.IntVar(&opt.interval, "k", 15, "")
	fs.Var(&opt.stunList, "s", "")
	fs.StringVar(&opt.keepaliveSrv, "h", "", "")
	fs.StringVar(&opt.notifySh, "e", "", "")
	fs.StringVar(&opt.bindIP, "i", "0.0.0.0", "")
	fs.IntVar(&opt.bindPort, "b", 0, "")
	fs.StringVar(&opt.method, "m", "", "")
	fs.StringVar(&opt.toIP, "t", "0.0.0.0", "")
	fs.IntVar(&opt.toPort, "p", 0, "")
	fs.BoolVar(&opt.keepRetry, "r", false, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if showVersion {
		fmt.Println("Natter " + natterVersion)
		return nil, errNatterExit
	}
	if showHelp {
		printUsage()
		return nil, errNatterExit
	}
	if check {
		if err := runNatterCheck(); err != nil {
			return nil, err
		}
		return nil, errNatterExit
	}
	return opt, nil
}

// runOnce 完整跑一遍 Natter：打洞、保活、转发、主循环。
// 返回 errNatterRetry 表示需要重来一次，errNatterExit 表示正常退出。
func runOnce(ctx context.Context, opt *options, showTitle bool) error {
	defer atExit.Run()

	if showTitle {
		slog.Info("Natter v" + natterVersion)
		if len(os.Args) == 1 {
			slog.Info("Tips: Use `--help` to see help messages")
		}
	}
	if err := checkDockerNetwork(); err != nil {
		return err
	}

	interval := time.Duration(opt.interval) * time.Second
	udp := opt.udpMode

	// -i 既可以是 IP 也可以是网卡名
	bindIP := opt.bindIP
	bindIface := ""
	if err := validateIP(bindIP); err != nil {
		bindIface, bindIP = bindIP, "0.0.0.0"
	}
	bindPort := opt.bindPort

	toIP, err := ipNormalize(opt.toIP)
	if err != nil {
		return err
	}
	toPort := opt.toPort

	// STUN 服务器列表
	stunList := []string(opt.stunList)
	if len(stunList) == 0 {
		stunList = append([]string{}, defaultStunServers...)
		if udp {
			stunList = append(defaultUDPStunServers, stunList...)
		} else {
			stunList = append(stunList, "turn.cloud-rtc.com:80")
		}
	}
	srvList := make([]addr, 0, len(stunList))
	for _, item := range stunList {
		host, port := splitHostPort(item, 3478)
		srvList = append(srvList, addr{host, port})
	}

	// 保活服务器
	keepaliveSrv := opt.keepaliveSrv
	if keepaliveSrv == "" {
		if udp {
			keepaliveSrv = "119.29.29.29"
		} else {
			keepaliveSrv = "www.baidu.com"
		}
	}
	defKeepalivePort := 80
	if udp {
		defKeepalivePort = 53
	}
	keepaliveHost, keepalivePort := splitHostPort(keepaliveSrv, defKeepalivePort)

	// 转发方法默认值
	method := opt.method
	if method == "" {
		switch {
		case toIP == "0.0.0.0" && toPort == 0 && bindIP == "0.0.0.0" && bindPort == 0 && bindIface == "":
			method = "test"
		case toIP == "0.0.0.0" && toPort == 0:
			method = "none"
		default:
			method = "socket"
		}
	}
	forwarder, err := newForwarder(method)
	if err != nil {
		return err
	}
	portTest := &PortTest{}

	// 1. 打洞：从 bindIP:bindPort 拿到映射后的公网地址
	stun, err := NewStunClient(srvList, bindIP, bindPort, bindIface, udp)
	if err != nil {
		return err
	}
	natterAddr, outerAddr, err := stun.GetMapping(ctx)
	if err != nil {
		return err
	}
	// 保活 socket 需要绑定实际的 IP 与端口，而不是通配和 0
	bindIP, bindPort = natterAddr.Host, natterAddr.Port

	// 2. 保活
	keepAlive := NewKeepAlive(keepaliveHost, keepalivePort, bindIP, bindPort, bindIface, udp)
	defer keepAlive.Disconnect()
	if err := keepAlive.KeepAlive(ctx); err != nil {
		return err
	}

	// 3. 保活连接建立后再取一次映射，验证网络是否稳定
	outerPrev := outerAddr
	natterAddr, outerAddr, err = stun.GetMapping(ctx)
	if err != nil {
		return err
	}
	if !outerAddr.equal(outerPrev) {
		slog.Warn("Network is unstable, or not full cone")
	}

	// 4. 修正转发目标：本机地址用实际内网 IP，端口缺省取公网端口
	if ip, err := inetAton(toIP); err == nil {
		if ip.IsLoopback() || ip.IsUnspecified() {
			toIP = natterAddr.Host
		}
	}
	if toPort == 0 {
		toPort = outerAddr.Port
	}
	// none / test 不是真实转发，目标地址直接取 Natter 自身地址
	if isNoopForwarder(forwarder) {
		toIP, toPort = natterAddr.Host, natterAddr.Port
	}
	toAddr := addr{toIP, toPort}

	if err := forwarder.StartForward(natterAddr.Host, natterAddr.Port, toIP, toPort, udp); err != nil {
		return err
	}
	atExit.Set(forwarder.StopForward)

	// 5. UPnP
	var upnp *UPnPClient
	upnpReady := false
	if opt.upnpEnabled {
		upnp = NewUPnPClient(natterAddr.Host, bindIface)
		slog.Info("")
		slog.Info("Scanning UPnP Devices...")
		router, err := upnp.DiscoverRouter(ctx)
		if err != nil {
			slog.Error("upnp: failed to discover router", "err", err)
		}
		if router != nil {
			slog.Info("[UPnP] Found router " + router.ipaddr)
			ok, err := upnp.Forward(ctx, "", bindPort, bindIP, bindPort, udp, opt.interval*3)
			if err != nil {
				slog.Error("upnp: failed to forward port", "err", err)
			} else {
				upnpReady = ok
			}
		}
	}

	// 6. 路由信息
	slog.Info("")
	routeStr := ""
	if !isNoopForwarder(forwarder) {
		routeStr += fmt.Sprintf("%s <--%s--> ", toAddr.uri(udp), method)
	}
	routeStr += fmt.Sprintf("%s <--Natter--> %s", natterAddr.uri(udp), outerAddr.uri(udp))
	slog.Info(routeStr)
	slog.Info("")

	// 7. 测试模式提示
	if method == "test" {
		scheme := "http"
		if udp {
			scheme = "udp"
		}
		slog.Info("Test mode in on.")
		slog.Info(fmt.Sprintf("Please check [ %s://%s ]", scheme, outerAddr))
		slog.Info("")
	}

	// 8. 通知脚本
	if opt.notifySh != "" {
		protocol := "tcp"
		if udp {
			protocol = "udp"
		}
		abs, err := filepath.Abs(opt.notifySh)
		if err != nil {
			return err
		}
		slog.Info("Calling script: " + opt.notifySh)
		cmd := exec.Command(abs, protocol, toIP, strconv.Itoa(toPort),
			outerAddr.Host, strconv.Itoa(outerAddr.Port))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			slog.Error("calling script failed", "err", err)
		}
	}

	// 9. 端口检测（仅 TCP）
	if !udp {
		ret1 := portTest.TestLAN(ctx, toAddr, "", "", true)
		_ = portTest.TestLAN(ctx, natterAddr, "", "", true)
		ret3 := portTest.TestLAN(ctx, outerAddr, natterAddr.Host, bindIface, true)
		ret4 := portTest.TestWAN(ctx, outerAddr, natterAddr.Host, bindIface, true)
		switch {
		case ret1 == portClosed:
			slog.Warn("!! Target port is closed !!")
		case ret1 == portOpen && ret3 == portClosed && ret4 == portClosed:
			slog.Warn("!! Hole punching failed !!")
		case ret3 == portOpen && ret4 == portClosed:
			slog.Warn("!! You may be behind a firewall !!")
		}
		slog.Info("")
		if opt.keepRetry && ret1 == portClosed {
			slog.Info(fmt.Sprintf("Retry after %d seconds...", opt.interval))
			if err := sleepCtx(ctx, interval); err != nil {
				return err
			}
			forwarder.StopForward()
			keepAlive.Disconnect()
			return fmt.Errorf("%w: target port is closed", errNatterRetry)
		}
	}

	// 10. 主循环
	needRecheck := false
	cnt := 0
	for {
		cnt = (cnt + 1) % 20
		if cnt == 0 {
			needRecheck = true
		}
		if needRecheck {
			slog.Debug("Start recheck")
			needRecheck = false
			if udp || portTest.TestLAN(ctx, outerAddr, natterAddr.Host, bindIface, false) == portClosed {
				_, outerCurr, err := stun.GetMapping(ctx)
				if err != nil {
					return err
				}
				if !outerCurr.equal(outerAddr) {
					forwarder.StopForward()
					keepAlive.Disconnect()
					if opt.exitWhenChanged {
						slog.Info("Natter is exiting because mapped address has changed")
						return fmt.Errorf("%w: mapped address has changed", errNatterExit)
					}
					return fmt.Errorf("%w: mapped address has changed", errNatterRetry)
				}
			}
		}

		ts := time.Now()
		if err := keepAlive.KeepAlive(ctx); err != nil {
			if errors.Is(err, syscall.EADDRNOTAVAIL) {
				if opt.exitWhenChanged {
					slog.Info("Natter is exiting because local IP address has changed")
					return fmt.Errorf("%w: local IP address has changed", errNatterExit)
				}
				return fmt.Errorf("%w: local IP address has changed", errNatterRetry)
			}
			if udp {
				slog.Debug("keep-alive: UDP response not received", "err", err)
			} else {
				slog.Error("keep-alive: connection broken", "err", err)
			}
			keepAlive.Disconnect()
			needRecheck = true
		}
		if upnpReady {
			if _, err := upnp.Renew(ctx); err != nil {
				slog.Error("upnp: failed to renew upnp", "err", err)
			}
		}
		if sleep := interval - time.Since(ts); sleep > 0 {
			if err := sleepCtx(ctx, sleep); err != nil {
				return err
			}
		}
	}
}

// validateOptions 校验命令行参数。
func validateOptions(opt *options) error {
	if err := validatePositive(opt.interval); err != nil {
		return err
	}
	for _, s := range opt.stunList {
		if err := validateAddrStr(s); err != nil {
			return err
		}
	}
	if err := validateAddrStr(opt.keepaliveSrv); err != nil {
		return err
	}
	if opt.notifySh != "" {
		if err := validateFilepath(opt.notifySh); err != nil {
			return err
		}
	}
	if err := validatePort(opt.bindPort); err != nil {
		return err
	}
	if err := validateIP(opt.toIP); err != nil {
		return err
	}
	return validatePort(opt.toPort)
}

func main() {
	setupLogger(false)

	opt, err := parseFlags()
	if err != nil {
		if errors.Is(err, errNatterExit) {
			return
		}
		slog.Error("参数解析失败", "err", err)
		os.Exit(2)
	}
	setupLogger(opt.verbose)
	if err := validateOptions(opt); err != nil {
		slog.Error("参数校验失败", "err", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// 收到退出信号时执行清理（停止转发、关闭保活连接）
	go func() {
		<-ctx.Done()
		atExit.Run()
		os.Exit(0)
	}()

	showTitle := true
	for {
		err := runOnce(ctx, opt, showTitle)
		switch {
		case err == nil:
			return
		case errors.Is(err, errNatterRetry):
		case errors.Is(err, errNatterExit), errors.Is(err, context.Canceled):
			return
		default:
			slog.Error("Natter 异常退出", "err", err)
			os.Exit(1)
		}
		showTitle = false
	}
}
