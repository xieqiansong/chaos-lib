package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// addr 对应 Python 版里的 (host, port) 二元组。
type addr struct {
	Host string
	Port int
}

// String 返回 host:port。
func (a addr) String() string { return net.JoinHostPort(a.Host, strconv.Itoa(a.Port)) }

// uri 返回 tcp://host:port 或 udp://host:port，对应 Python 的 addr_to_uri。
func (a addr) uri(udp bool) string {
	if udp {
		return "udp://" + a.String()
	}
	return "tcp://" + a.String()
}

// equal 判断两个地址是否相同（Python 版直接比较元组）。
func (a addr) equal(b addr) bool { return a.Host == b.Host && a.Port == b.Port }

// networkName 返回 "tcp4" / "udp4"。
func networkName(udp bool) string {
	if udp {
		return "udp4"
	}
	return "tcp4"
}

// sockOpts 汇总 Python 版 socket_set_opt 的参数。
type sockOpts struct {
	reuse    bool          // 设置 SO_REUSEADDR / SO_REUSEPORT
	bindIP   string        // 本地绑定 IP，空表示不限制
	bindPort int           // 本地绑定端口，0 表示由内核分配
	iface    string        // 绑定的网卡名，仅 Linux 支持
	timeout  time.Duration // 连接超时，<=0 表示不设置
	udp      bool
}

// localAddr 生成本地绑定地址；IP 与端口都为「空/0」时返回 nil（不绑定）。
func (o sockOpts) localAddr() net.Addr {
	if o.bindIP == "" && o.bindPort == 0 {
		return nil
	}
	ip := net.ParseIP(o.bindIP) // 空串 -> nil，即通配地址
	if o.udp {
		return &net.UDPAddr{IP: ip, Port: o.bindPort}
	}
	return &net.TCPAddr{IP: ip, Port: o.bindPort}
}

// controlFn 生成 socket 选项回调。
//
// 注意：Go 的 Dialer.Control / ListenConfig.Control 都在 bind() 之前被调用
// （见 net/sock_posix.go 的 dial / listenStream / listenDatagram），
// 因此 SO_REUSEPORT 与 SO_BINDTODEVICE 能如期生效。
func controlFn(o sockOpts) func(network, address string, c syscall.RawConn) error {
	if !o.reuse && o.iface == "" {
		return nil
	}
	return func(_, _ string, c syscall.RawConn) error {
		var sockErr error
		if err := c.Control(func(fd uintptr) {
			if o.reuse {
				sockErr = setReuse(fd)
			}
			if sockErr == nil && o.iface != "" {
				sockErr = bindToDevice(fd, o.iface)
			}
		}); err != nil {
			return err
		}
		return sockErr
	}
}

// newDialer 构造带 socket 选项的 net.Dialer。
func newDialer(o sockOpts) *net.Dialer {
	d := &net.Dialer{LocalAddr: o.localAddr()}
	if o.timeout > 0 {
		d.Timeout = o.timeout
	}
	if fn := controlFn(o); fn != nil {
		d.Control = fn
	}
	return d
}

// newListenConfig 构造带 socket 选项的 net.ListenConfig，用于监听型 socket。
func newListenConfig(o sockOpts) *net.ListenConfig {
	lc := &net.ListenConfig{}
	if fn := controlFn(o); fn != nil {
		lc.Control = fn
	}
	return lc
}

// listenAddr 生成监听地址串；host 为空表示监听全部网卡。
func listenAddr(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// addrFromNetAddr 把 net.Addr 转成 addr。
func addrFromNetAddr(na net.Addr) (addr, error) {
	switch v := na.(type) {
	case *net.TCPAddr:
		return addr{v.IP.String(), v.Port}, nil
	case *net.UDPAddr:
		return addr{v.IP.String(), v.Port}, nil
	}
	host, portStr, err := net.SplitHostPort(na.String())
	if err != nil {
		return addr{}, fmt.Errorf("cannot parse address %q: %w", na.String(), err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return addr{}, fmt.Errorf("cannot parse port %q: %w", portStr, err)
	}
	return addr{host, port}, nil
}

// inetAton 兼容 Python socket.inet_aton 的宽松解析（"10.1" -> 10.0.0.1）。
func inetAton(s string) (net.IP, error) {
	if s == "" {
		return nil, fmt.Errorf("invalid IP address: %q", s)
	}
	parts := strings.Split(s, ".")
	if len(parts) > 4 {
		return nil, fmt.Errorf("invalid IP address: %q", s)
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address: %q", s)
		}
		vals[i] = v
	}
	var b [4]byte
	switch len(parts) {
	case 1:
		if vals[0] > math.MaxUint32 {
			return nil, fmt.Errorf("invalid IP address: %q", s)
		}
		binary.BigEndian.PutUint32(b[:], uint32(vals[0]))
	case 2:
		if vals[0] > 255 || vals[1] > 0xFFFFFF {
			return nil, fmt.Errorf("invalid IP address: %q", s)
		}
		b[0] = byte(vals[0])
		b[1], b[2], b[3] = byte(vals[1]>>16), byte(vals[1]>>8), byte(vals[1])
	case 3:
		if vals[0] > 255 || vals[1] > 255 || vals[2] > 0xFFFF {
			return nil, fmt.Errorf("invalid IP address: %q", s)
		}
		b[0], b[1] = byte(vals[0]), byte(vals[1])
		binary.BigEndian.PutUint16(b[2:], uint16(vals[2]))
	case 4:
		for i, v := range vals {
			if v > 255 {
				return nil, fmt.Errorf("invalid IP address: %q", s)
			}
			b[i] = byte(v)
		}
	}
	return net.IPv4(b[0], b[1], b[2], b[3]), nil
}

// ipNormalize 把 IP 规范化成点分十进制。
func ipNormalize(s string) (string, error) {
	ip, err := inetAton(s)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// validateIP 校验 IP 地址。
func validateIP(s string) error {
	_, err := inetAton(s)
	return err
}

// validatePortStr 校验端口号字符串。
func validatePortStr(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid port number: %s", s)
	}
	return nil
}

// validatePort 校验端口号。
func validatePort(n int) error {
	if n < 0 || n > 65535 {
		return fmt.Errorf("invalid port number: %d", n)
	}
	return nil
}

// validateAddrStr 校验 "host[:port]" 形式的地址。
func validateAddrStr(s string) error {
	if i := strings.Index(s, ":"); i >= 0 {
		return validatePortStr(s[i+1:])
	}
	return nil
}

// validatePositive 校验正整数。
func validatePositive(n int) error {
	if n <= 0 {
		return fmt.Errorf("not a positive integer: %d", n)
	}
	return nil
}

// validateFilepath 校验文件存在。
func validateFilepath(p string) error {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return fmt.Errorf("file not found: %s", p)
	}
	return nil
}

// urlRe 匹配 http://host[:port][/path]，对应 Python 的 split_url。
var urlRe = regexp.MustCompile(`^http://([^\[\]:/]+)(?::([0-9]+))?(/\S*)?$`)

// splitURL 拆分 URL 为 host / port / path，端口缺省 80，路径缺省 "/"。
func splitURL(u string) (host string, port int, path string, err error) {
	m := urlRe.FindStringSubmatch(u)
	if m == nil {
		return "", 0, "", fmt.Errorf("unsupported URL: %s", u)
	}
	host = m[1]
	port = 80
	if m[2] != "" {
		port, err = strconv.Atoi(m[2])
		if err != nil {
			return "", 0, "", fmt.Errorf("unsupported URL: %s", u)
		}
	}
	path = m[3]
	if path == "" {
		path = "/"
	}
	return host, port, path, nil
}

// fullURL 把以 "/" 开头的相对 URL 基于参照 URL 补全，对应 Python 的 full_url。
func fullURL(u, ref string) string {
	if !strings.HasPrefix(u, "/") {
		return u
	}
	host, port, _, err := splitURL(ref)
	if err != nil {
		return u
	}
	return "http://" + listenAddr(host, port) + u
}

// splitHostPort 按 "host[:port]" 拆分，端口缺省用 def。
func splitHostPort(s string, def int) (string, int) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		if p, err := strconv.Atoi(parts[1]); err == nil {
			return parts[0], p
		}
	}
	return parts[0], def
}

// isClosedErr 判断是否是「socket 已关闭」类错误，用于静默退出后台 goroutine。
func isClosedErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EBADF) ||
		errors.Is(err, syscall.EINTR)
}

// isTimeoutErr 判断是否是超时错误。
func isTimeoutErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// checkDockerNetwork 检测 Docker 网络是否满足要求（仅 Linux 生效）。
func checkDockerNetwork() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return nil
	}
	rawMAC, err := os.ReadFile("/sys/class/net/eth0/address")
	if err != nil {
		return nil
	}
	macAddr := strings.TrimSpace(string(rawMAC))

	hostname, err := os.Hostname()
	if err != nil {
		return nil
	}
	ips, err := net.LookupHost(hostname)
	if err != nil || len(ips) == 0 {
		slog.Warn("check-docker-network: cannot resolve hostname", "hostname", hostname)
		return nil
	}
	ip := net.ParseIP(ips[0]).To4()
	if ip == nil {
		return nil
	}
	dockerMAC := fmt.Sprintf("02:42:%02x:%02x:%02x:%02x", ip[0], ip[1], ip[2], ip[3])
	if strings.EqualFold(macAddr, dockerMAC) {
		return errors.New("Docker's `--net=host` option is required")
	}

	rawRelease, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(string(rawRelease)), "-")
	suffix := strings.ToLower(parts[len(parts)-1])
	if (suffix == "linuxkit" || suffix == "wsl2") && strings.EqualFold(hostname, "docker-desktop") {
		return errors.New("network from Docker Desktop is not supported")
	}
	return nil
}
