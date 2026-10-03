package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// 端口检测结果的三种取值，与 Python 版一致。
const (
	portOpen    = 1
	portClosed  = -1
	portUnknown = 0
)

// PortTest 检测端口在 LAN / WAN 侧是否可达。
type PortTest struct{}

// logAt 按 info 参数选择日志级别：启动时用 info，主循环里用 debug。
func logAt(info bool) func(string) {
	if info {
		return func(s string) { slog.Info(s) }
	}
	return func(s string) { slog.Debug(s) }
}

// TestLAN 从局域网侧测试端口：1 开放 / -1 关闭 / 0 未知。
func (p *PortTest) TestLAN(ctx context.Context, a addr, sourceIP, iface string, info bool) int {
	printStatus := logAt(info)
	o := sockOpts{bindIP: sourceIP, iface: iface, timeout: time.Second}
	conn, err := newDialer(o).DialContext(ctx, "tcp4", a.String())
	if err == nil {
		_ = conn.Close()
		printStatus(fmt.Sprintf("LAN > %-21s [ OPEN ]", a.String()))
		return portOpen
	}
	if isTimeoutErr(err) {
		// 超时无法判定：可能是防火墙丢包
		printStatus(fmt.Sprintf("LAN > %-21s [ UNKNOWN ]", a.String()))
		slog.Debug("cannot test port from LAN", "addr", a.String(), "err", err)
		return portUnknown
	}
	printStatus(fmt.Sprintf("LAN > %-21s [ CLOSED ]", a.String()))
	return portClosed
}

// TestWAN 从公网侧测试端口。注意只使用 addr 中的端口号，公网 IP 会被忽略。
func (p *PortTest) TestWAN(ctx context.Context, a addr, sourceIP, iface string, info bool) int {
	printStatus := logAt(info)
	ret01 := p.testIfconfigCo(ctx, a.Port, sourceIP, iface)
	if ret01 == portOpen {
		printStatus(fmt.Sprintf("WAN > %-21s [ OPEN ]", a.String()))
		return portOpen
	}
	ret02 := p.testTransmission(ctx, a.Port, sourceIP, iface)
	if ret02 == portOpen {
		printStatus(fmt.Sprintf("WAN > %-21s [ OPEN ]", a.String()))
		return portOpen
	}
	if ret01 == portClosed && ret02 == portClosed {
		printStatus(fmt.Sprintf("WAN > %-21s [ CLOSED ]", a.String()))
		return portClosed
	}
	printStatus(fmt.Sprintf("WAN > %-21s [ UNKNOWN ]", a.String()))
	return portUnknown
}

// testIfconfigCo 通过 ifconfig.co 的端口检测接口探测（repo: mpolden/echoip）。
func (p *PortTest) testIfconfigCo(ctx context.Context, port int, sourceIP, iface string) int {
	req := fmt.Sprintf(
		"GET /port/%d HTTP/1.0\r\n"+
			"Host: ifconfig.co\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"Connection: close\r\n"+
			"\r\n", port)
	o := sockOpts{bindIP: sourceIP, iface: iface, timeout: 8 * time.Second}
	resp, err := rawHTTP(ctx, o, addr{"ifconfig.co", 80}, []byte(req))
	if err != nil {
		slog.Debug("cannot test port from ifconfig.co", "port", port, "err", err)
		return portUnknown
	}
	slog.Debug("port-test: ifconfig.co: " + string(resp))
	content, ok := httpBody(resp)
	if !ok {
		return portUnknown
	}
	var dat struct {
		Reachable bool `json:"reachable"`
	}
	if err := json.Unmarshal(content, &dat); err != nil {
		return portUnknown
	}
	if dat.Reachable {
		return portOpen
	}
	return portClosed
}

// testTransmission 通过 Transmission 的端口检测接口探测。
// repo: https://github.com/transmission/portcheck
func (p *PortTest) testTransmission(ctx context.Context, port int, sourceIP, iface string) int {
	req := fmt.Sprintf(
		"GET /%d HTTP/1.0\r\n"+
			"Host: portcheck.transmissionbt.com\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"Connection: close\r\n"+
			"\r\n", port)
	o := sockOpts{bindIP: sourceIP, iface: iface, timeout: 8 * time.Second}
	resp, err := rawHTTP(ctx, o, addr{"portcheck.transmissionbt.com", 80}, []byte(req))
	if err != nil {
		slog.Debug("cannot test port from portcheck.transmissionbt.com", "port", port, "err", err)
		return portUnknown
	}
	slog.Debug("port-test: portcheck.transmissionbt.com: " + string(resp))
	content, ok := httpBody(resp)
	if !ok {
		return portUnknown
	}
	switch string(bytes.TrimSpace(content)) {
	case "1":
		return portOpen
	case "0":
		return portClosed
	}
	return portUnknown
}

// rawHTTP 用裸 TCP 完成一次 HTTP 请求并读完整响应。
// 因为要精确控制源地址 / 网卡绑定，所以不走 net/http。
func rawHTTP(ctx context.Context, o sockOpts, target addr, req []byte) ([]byte, error) {
	conn, err := newDialer(o).DialContext(ctx, "tcp4", target.String())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	timeout := o.timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			if isTimeoutErr(err) {
				return nil, err
			}
			if errors.Is(err, io.EOF) {
				return out.Bytes(), nil
			}
			return nil, err
		}
	}
}

// httpBody 去掉 HTTP 响应头，返回消息体。
func httpBody(resp []byte) ([]byte, bool) {
	idx := bytes.Index(resp, []byte("\r\n\r\n"))
	if idx < 0 {
		return nil, false
	}
	return resp[idx+4:], true
}
