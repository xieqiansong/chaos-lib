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

type addr struct {
	Host string
	Port int
}

func (a addr) String() string { return net.JoinHostPort(a.Host, strconv.Itoa(a.Port)) }

func (a addr) uri(udp bool) string {
	if udp {
		return "udp://" + a.String()
	}
	return "tcp://" + a.String()
}

func (a addr) equal(b addr) bool { return a.Host == b.Host && a.Port == b.Port }

func networkName(udp bool) string {
	if udp {
		return "udp4"
	}
	return "tcp4"
}

type sockOpts struct {
	reuse    bool
	bindIP   string
	bindPort int
	iface    string
	timeout  time.Duration
	udp      bool
}

func (o sockOpts) localAddr() net.Addr {
	if o.bindIP == "" && o.bindPort == 0 {
		return nil
	}
	ip := net.ParseIP(o.bindIP)
	if o.udp {
		return &net.UDPAddr{IP: ip, Port: o.bindPort}
	}
	return &net.TCPAddr{IP: ip, Port: o.bindPort}
}

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

func newListenConfig(o sockOpts) *net.ListenConfig {
	lc := &net.ListenConfig{}
	if fn := controlFn(o); fn != nil {
		lc.Control = fn
	}
	return lc
}

func listenAddr(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

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

func ipNormalize(s string) (string, error) {
	ip, err := inetAton(s)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

func validateIP(s string) error {
	_, err := inetAton(s)
	return err
}

func validatePortStr(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid port number: %s", s)
	}
	return nil
}

func validatePort(n int) error {
	if n < 0 || n > 65535 {
		return fmt.Errorf("invalid port number: %d", n)
	}
	return nil
}

func validateAddrStr(s string) error {
	if i := strings.Index(s, ":"); i >= 0 {
		return validatePortStr(s[i+1:])
	}
	return nil
}

func validatePositive(n int) error {
	if n <= 0 {
		return fmt.Errorf("not a positive integer: %d", n)
	}
	return nil
}

func validateFilepath(p string) error {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return fmt.Errorf("file not found: %s", p)
	}
	return nil
}

// urlRe 匹配 http://host[:port][/path]，对应 Python 的 split_url。
var urlRe = regexp.MustCompile(`^http://([^\[\]:/]+)(?::([0-9]+))?(/\S*)?$`)

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

func splitHostPort(s string, def int) (string, int) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		if p, err := strconv.Atoi(parts[1]); err == nil {
			return parts[0], p
		}
	}
	return parts[0], def
}

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

func isTimeoutErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

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
