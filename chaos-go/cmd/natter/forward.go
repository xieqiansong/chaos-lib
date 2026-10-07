package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	fwdBufferSize    = 8192
	fwdUDPTimeout    = 60 * time.Second
	fwdMaxConcurrent = 128
)

// Forwarder 端口转发统一接口。
type Forwarder interface {
	StartForward(ip string, port int, toIP string, toPort int, udp bool) error
	StopForward()
}

// noopForwarder 标记非真实转发（none / test）。
type noopForwarder interface{ isNoop() }

func isNoopForwarder(f Forwarder) bool {
	_, ok := f.(noopForwarder)
	return ok
}

func checkForwardTarget(ip string, port int, toIP string, toPort int) error {
	from := addr{ip, port}
	if from.equal(addr{toIP, toPort}) {
		return fmt.Errorf("cannot forward to the same address %s", from)
	}
	return nil
}

// ForwardNone 空实现。
type ForwardNone struct{}

func (*ForwardNone) isNoop() {}
func (*ForwardNone) StartForward(string, int, string, int, bool) error {
	return nil
}
func (*ForwardNone) StopForward() {}

// ForwardTestServer 启动测试应答服务。
type ForwardTestServer struct {
	mu     sync.Mutex
	closer io.Closer
}

func (*ForwardTestServer) isNoop() {}

// StartForward 启动测试服务（TCP HTML / UDP 回显）。
func (f *ForwardTestServer) StartForward(ip string, port int, _ string, _ int, udp bool) error {
	lc := newListenConfig(sockOpts{reuse: true})
	bind := listenAddr("", port)
	slog.Debug("fwd-test: Starting test server", "addr", addr{ip, port}.uri(udp))

	ctx := context.Background()
	if udp {
		pc, err := lc.ListenPacket(ctx, "udp4", bind)
		if err != nil {
			return err
		}
		f.mu.Lock()
		f.closer = pc
		f.mu.Unlock()
		go f.serveUDP(pc)
		return nil
	}
	ln, err := lc.Listen(ctx, "tcp4", bind)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.closer = ln
	f.mu.Unlock()
	go f.serveHTTP(ln)
	return nil
}

func (f *ForwardTestServer) serveHTTP(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if !isClosedErr(err) {
				slog.Error("fwd-test: accept 失败", "err", err)
			}
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(3 * time.Second))
			buf := make([]byte, fwdBufferSize)
			_, _ = c.Read(buf)
			content := "<html><body><h1>It works!</h1><hr/>Natter</body></html>"
			_, _ = fmt.Fprintf(c,
				"HTTP/1.1 200 OK\r\n"+
					"Content-Type: text/html\r\n"+
					"Content-Length: %d\r\n"+
					"Connection: close\r\n"+
					"Server: Natter\r\n"+
					"\r\n"+
					"%s\r\n", len(content), content)
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.CloseWrite()
			}
		}(conn)
	}
}

func (f *ForwardTestServer) serveUDP(pc net.PacketConn) {
	buf := make([]byte, fwdBufferSize)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if !isClosedErr(err) {
				slog.Error("fwd-test: recvfrom 失败", "err", err)
			}
			return
		}
		slog.Debug("fwd-test: got client", "addr", from, "bytes", n)
		_, _ = pc.WriteTo([]byte("It works! - Natter\r\n"), from)
	}
}

func (f *ForwardTestServer) StopForward() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closer != nil {
		slog.Debug("fwd-test: Stopping test server")
		_ = f.closer.Close()
		f.closer = nil
	}
}

// ForwardSocket 纯 Go TCP/UDP 转发。
type ForwardSocket struct {
	mu       sync.Mutex
	closer   io.Closer
	outbound addr
	sem      chan struct{}
}

func NewForwardSocket() *ForwardSocket {
	return &ForwardSocket{sem: make(chan struct{}, fwdMaxConcurrent)}
}

func (f *ForwardSocket) StartForward(ip string, port int, toIP string, toPort int, udp bool) error {
	if err := checkForwardTarget(ip, port, toIP, toPort); err != nil {
		return err
	}
	f.mu.Lock()
	f.outbound = addr{toIP, toPort}
	f.mu.Unlock()

	lc := newListenConfig(sockOpts{reuse: true})
	bind := listenAddr("", port)
	slog.Debug("fwd-socket: Starting socket",
		"from", addr{ip, port}.uri(udp), "to", addr{toIP, toPort}.uri(udp))

	ctx := context.Background()
	if udp {
		pc, err := lc.ListenPacket(ctx, "udp4", bind)
		if err != nil {
			return err
		}
		f.mu.Lock()
		f.closer = pc
		f.mu.Unlock()
		go f.udpRecvFrom(pc)
		return nil
	}
	ln, err := lc.Listen(ctx, "tcp4", bind)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.closer = ln
	f.mu.Unlock()
	go f.tcpListen(ln)
	return nil
}

func (f *ForwardSocket) tcpListen(ln net.Listener) {
	for {
		inbound, err := ln.Accept()
		if err != nil {
			if !isClosedErr(err) {
				slog.Error("fwd-socket: socket listening goroutine is exiting", "err", err)
			}
			return
		}
		select {
		case f.sem <- struct{}{}:
		default:
			slog.Error("fwd-socket: too many connections")
			_ = inbound.Close()
			continue
		}
		f.mu.Lock()
		target := f.outbound
		f.mu.Unlock()

		outbound, err := net.DialTimeout("tcp4", target.String(), 3*time.Second)
		if err != nil {
			slog.Error("fwd-socket: cannot forward port", "err", err)
			_ = inbound.Close()
			<-f.sem
			continue
		}
		go func() {
			defer func() { <-f.sem }()
			relay(inbound, outbound)
		}()
	}
}

func (f *ForwardSocket) udpRecvFrom(pc net.PacketConn) {
	var mu sync.Mutex
	socks := map[string]*net.UDPConn{}
	f.mu.Lock()
	target := f.outbound
	f.mu.Unlock()

	buf := make([]byte, fwdBufferSize)
	for {
		n, clientAddr, err := pc.ReadFrom(buf)
		if err != nil {
			if !isClosedErr(err) {
				slog.Error("fwd-socket: socket recvfrom goroutine is exiting", "err", err)
			}
			mu.Lock()
			for _, s := range socks {
				_ = s.Close()
			}
			mu.Unlock()
			return
		}
		key := clientAddr.String()
		mu.Lock()
		s := socks[key]
		mu.Unlock()

		if s == nil {
			select {
			case f.sem <- struct{}{}:
			default:
				slog.Error("fwd-socket: too many connections")
				continue
			}
			conn, err := net.Dial("udp4", target.String())
			if err != nil {
				slog.Error("fwd-socket: cannot forward port", "err", err)
				<-f.sem
				continue
			}
			s = conn.(*net.UDPConn)
			_ = s.SetDeadline(time.Now().Add(fwdUDPTimeout))
			mu.Lock()
			socks[key] = s
			mu.Unlock()
			go func(s *net.UDPConn, clientAddr net.Addr, key string) {
				defer func() {
					mu.Lock()
					delete(socks, key)
					mu.Unlock()
					_ = s.Close()
					<-f.sem
				}()
				b := make([]byte, fwdBufferSize)
				for {
					n, err := s.Read(b)
					if n > 0 {
						_, _ = pc.WriteTo(b[:n], clientAddr)
					}
					if err != nil {
						return
					}
				}
			}(s, clientAddr, key)
		}

		if n > 0 {
			if _, err := s.Write(buf[:n]); err != nil {
				mu.Lock()
				if cur := socks[key]; cur == s {
					delete(socks, key)
				}
				mu.Unlock()
				_ = s.Close()
			}
		} else {
			mu.Lock()
			delete(socks, key)
			mu.Unlock()
			_ = s.Close()
		}
	}
}

func (f *ForwardSocket) StopForward() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closer != nil {
		slog.Debug("fwd-socket: Stopping socket")
		_ = f.closer.Close()
		f.closer = nil
	}
}

// relay 双向搬运两条连接。
func relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer b.Close()
		if _, err := io.Copy(b, a); err != nil && !isClosedErr(err) {
			slog.Debug("fwd-socket: forwarding stopped", "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		defer a.Close()
		if _, err := io.Copy(a, b); err != nil && !isClosedErr(err) {
			slog.Debug("fwd-socket: forwarding stopped", "err", err)
		}
	}()
	wg.Wait()
}
