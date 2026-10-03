package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"time"
)

// stunMagicCookie 是 RFC 5389 规定的固定 magic cookie。
const stunMagicCookie = 0x2112a442

// stunAttrMappedAddress / stunAttrXORMappedAddress 是两种携带映射地址的 STUN 属性。
const (
	stunAttrMappedAddress    = 0x0001
	stunAttrXORMappedAddress = 0x0020
)

// serverUnavailable 表示当前 STUN 服务器不可用，调用方应轮换到下一个。
type serverUnavailable struct{ err error }

func (e *serverUnavailable) Error() string { return fmt.Sprintf("STUN server unavailable: %v", e.err) }
func (e *serverUnavailable) Unwrap() error { return e.err }

// StunClient 通过 STUN 绑定请求获取 NAT 映射后的公网地址。
// ref: https://www.rfc-editor.org/rfc/rfc5389
type StunClient struct {
	servers    []addr
	sourceHost string
	sourcePort int
	iface      string
	udp        bool
}

// NewStunClient 创建 STUN 客户端，服务器列表为空时报错。
func NewStunClient(servers []addr, sourceHost string, sourcePort int, iface string, udp bool) (*StunClient, error) {
	if len(servers) == 0 {
		return nil, errors.New("STUN server list is empty")
	}
	return &StunClient{
		servers:    servers,
		sourceHost: sourceHost,
		sourcePort: sourcePort,
		iface:      iface,
		udp:        udp,
	}, nil
}

// GetMapping 依次尝试 STUN 服务器，成功返回 (内网地址, 公网地址)。
// 一轮全部失败时等待 10 秒再重来，与 Python 版一致。
func (c *StunClient) GetMapping(ctx context.Context) (addr, addr, error) {
	first := c.servers[0]
	for {
		inner, outer, err := c.getMapping(ctx)
		if err == nil {
			return inner, outer, nil
		}
		slog.Warn("stun: STUN server is unavailable", "server", c.servers[0].uri(c.udp), "err", err)
		// 轮换：队首服务器挪到队尾
		head := c.servers[0]
		c.servers = append(c.servers[1:], head)
		if c.servers[0].equal(first) {
			slog.Error("stun: No STUN server is available right now")
			if err := sleepCtx(ctx, 10*time.Second); err != nil {
				return addr{}, addr{}, err
			}
		}
	}
}

// getMapping 向队首 STUN 服务器发一次绑定请求并解析响应。
func (c *StunClient) getMapping(ctx context.Context) (addr, addr, error) {
	stun := c.servers[0]
	o := sockOpts{
		reuse:    true,
		bindIP:   c.sourceHost,
		bindPort: c.sourcePort,
		iface:    c.iface,
		timeout:  3 * time.Second,
		udp:      c.udp,
	}
	conn, err := newDialer(o).DialContext(ctx, networkName(c.udp), stun.String())
	if err != nil {
		return addr{}, addr{}, &serverUnavailable{err}
	}
	defer conn.Close()

	// 连接建立后本地地址才是确定的（端口为 0 时由内核分配）
	inner, err := addrFromNetAddr(conn.LocalAddr())
	if err != nil {
		return addr{}, addr{}, &serverUnavailable{err}
	}
	c.sourceHost, c.sourcePort = inner.Host, inner.Port

	deadline := time.Now().Add(3 * time.Second)
	if err := conn.SetDeadline(deadline); err != nil {
		return addr{}, addr{}, &serverUnavailable{err}
	}

	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:2], 0x0001) // Binding Request
	binary.BigEndian.PutUint16(req[2:4], 0x0000) // message length
	binary.BigEndian.PutUint32(req[4:8], stunMagicCookie)
	copy(req[8:12], []byte("NATR"))
	binary.BigEndian.PutUint32(req[12:16], rand.Uint32())
	binary.BigEndian.PutUint32(req[16:20], rand.Uint32())
	if _, err := conn.Write(req); err != nil {
		return addr{}, addr{}, &serverUnavailable{err}
	}

	buff := make([]byte, 1500)
	n, err := conn.Read(buff)
	if err != nil {
		return addr{}, addr{}, &serverUnavailable{err}
	}
	if n < 20 {
		return addr{}, addr{}, &serverUnavailable{errors.New("invalid STUN response")}
	}

	payload := buff[20:n]
	for len(payload) >= 4 {
		attrType := binary.BigEndian.Uint16(payload[0:2])
		attrLen := int(binary.BigEndian.Uint16(payload[2:4]))
		if len(payload) < 4+attrLen {
			break
		}
		if attrType == stunAttrMappedAddress || attrType == stunAttrXORMappedAddress {
			if attrLen < 8 {
				return addr{}, addr{}, &serverUnavailable{errors.New("invalid STUN attribute")}
			}
			val := payload[4 : 4+attrLen]
			// 1 字节保留位 + 1 字节地址族 + 2 字节端口 + 4 字节 IP
			port := binary.BigEndian.Uint16(val[2:4])
			ip := binary.BigEndian.Uint32(val[4:8])
			if attrType == stunAttrXORMappedAddress {
				port ^= 0x2112
				ip ^= stunMagicCookie
			}
			outer := addr{
				Host: net.IPv4(byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip)).String(),
				Port: int(port),
			}
			slog.Debug("stun: Got address",
				"outer", outer.uri(c.udp), "server", stun.uri(c.udp), "source", inner.uri(c.udp))
			return inner, outer, nil
		}
		payload = payload[4+attrLen:]
	}
	return addr{}, addr{}, &serverUnavailable{errors.New("invalid STUN response")}
}
