package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"runtime"
	"time"
)

const keepAliveTimeout = 3 * time.Second

// KeepAlive 维持 NAT 映射：周期性向保活服务器发包并读取响应。
type KeepAlive struct {
	host       string
	port       int
	sourceHost string
	sourcePort int
	iface      string
	udp        bool

	conn   net.Conn
	reconn bool
}

// NewKeepAlive 创建保活对象。
func NewKeepAlive(host string, port int, sourceHost string, sourcePort int, iface string, udp bool) *KeepAlive {
	return &KeepAlive{
		host: host, port: port,
		sourceHost: sourceHost, sourcePort: sourcePort,
		iface: iface, udp: udp,
	}
}

// Disconnect 关闭连接并标记下次需要重连。
func (k *KeepAlive) Disconnect() {
	if k.conn != nil {
		_ = k.conn.Close()
		k.conn = nil
		k.reconn = true
	}
}

func (k *KeepAlive) connect(ctx context.Context) error {
	o := sockOpts{
		reuse:    true,
		bindIP:   k.sourceHost,
		bindPort: k.sourcePort,
		iface:    k.iface,
		timeout:  keepAliveTimeout,
		udp:      k.udp,
	}
	conn, err := newDialer(o).DialContext(ctx, networkName(k.udp), listenAddr(k.host, k.port))
	if err != nil {
		return err
	}
	k.conn = conn
	if !k.udp {
		slog.Debug("keep-alive: Connected to host", "host", addr{k.host, k.port}.uri(k.udp))
		if k.reconn {
			slog.Info("keep-alive: connection restored")
		}
	}
	k.reconn = false
	return nil
}

// KeepAlive 执行一次保活：必要时先建连，然后发送并读取响应。
func (k *KeepAlive) KeepAlive(ctx context.Context) error {
	if k.conn == nil {
		if err := k.connect(ctx); err != nil {
			return err
		}
	}
	var err error
	if k.udp {
		err = k.keepAliveUDP()
	} else {
		err = k.keepAliveTCP()
	}
	if err != nil {
		return err
	}
	slog.Debug("keep-alive: OK")
	return nil
}

func (k *KeepAlive) keepAliveTCP() error {
	req := fmt.Sprintf(
		"HEAD /natter-keep-alive HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: curl/8.0.0 (Natter)\r\n"+
			"Accept: */*\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n", k.host)
	if err := k.conn.SetDeadline(time.Now().Add(keepAliveTimeout)); err != nil {
		return err
	}
	if _, err := k.conn.Write([]byte(req)); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	got := false
	for {
		n, err := k.conn.Read(buf)
		if n > 0 {
			got = true
		}
		if err != nil {
			if isTimeoutErr(err) {
				if !got {
					return err
				}
				return nil
			}
			if errors.Is(err, io.EOF) {
				return errors.New("keep-alive server closed connection")
			}
			return err
		}
	}
}

// keepAliveUDP 发送一个 DNS 查询，读到超时为止。
func (k *KeepAlive) keepAliveUDP() error {
	qname := "\x09keepalive\x06natter\x00" // 9keepalive 6natter 0
	req := make([]byte, 12+len(qname)+4)
	binary.BigEndian.PutUint16(req[0:2], uint16(rand.Intn(65536))) // 事务 ID
	binary.BigEndian.PutUint16(req[2:4], 0x0100)                   // 标准查询
	binary.BigEndian.PutUint16(req[4:6], 0x0001)                   // QDCOUNT
	// ANCOUNT / NSCOUNT / ARCOUNT 保持 0
	copy(req[12:], qname)
	binary.BigEndian.PutUint16(req[12+len(qname):], 0x0001) // QTYPE: A
	binary.BigEndian.PutUint16(req[14+len(qname):], 0x0001) // QCLASS: IN

	if err := k.conn.SetDeadline(time.Now().Add(keepAliveTimeout)); err != nil {
		return err
	}
	if _, err := k.conn.Write(req); err != nil {
		return err
	}
	buf := make([]byte, 1500)
	got := false
	for {
		n, err := k.conn.Read(buf)
		if n > 0 {
			got = true
		}
		if err != nil {
			if isTimeoutErr(err) {
				if !got {
					return err
				}
				// fix: Windows 上保活 socket 会导致 STUN socket 超时
				if runtime.GOOS == "windows" {
					k.Disconnect()
				}
				return nil
			}
			if errors.Is(err, io.EOF) {
				return errors.New("keep-alive server closed connection")
			}
			return err
		}
	}
}
