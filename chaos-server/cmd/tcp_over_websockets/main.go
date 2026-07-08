package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TcpToWsForwarder struct {
	targetHost string
	targetPort string
	conn       net.Conn
	wsConn     *websocket.Conn
	mu         sync.Mutex
}

func NewTcpToWsForwarder(targetHost, targetPort string, wsConn *websocket.Conn) *TcpToWsForwarder {
	return &TcpToWsForwarder{
		targetHost: targetHost,
		targetPort: targetPort,
		wsConn:     wsConn,
	}
}

func (f *TcpToWsForwarder) Run(ctx context.Context) {
	addr := fmt.Sprintf("%s:%s", f.targetHost, f.targetPort)
	slog.Info("开始建立 TCP 连接", "targetHost", f.targetHost, "targetPort", f.targetPort)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		slog.Error("TCP 连接失败", "addr", addr, "err", err)
		return
	}
	f.conn = conn
	defer conn.Close()

	slog.Info("TCP 连接成功", "addr", addr)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		f.forwardTcpToWs(ctx)
	}()

	go func() {
		defer wg.Done()
		f.forwardWsToTcp(ctx)
	}()

	wg.Wait()
	slog.Info("TCP 连接结束", "addr", addr)
}

func (f *TcpToWsForwarder) forwardTcpToWs(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := f.conn.Read(buf)
		if err != nil {
			slog.Error("TCP 读取失败", "err", err)
			return
		}

		slog.Debug("TCP 收到数据", "data", hex.EncodeToString(buf[:n]))

		f.mu.Lock()
		err = f.wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
		f.mu.Unlock()

		if err != nil {
			slog.Error("WebSocket 写入失败", "err", err)
			return
		}
	}
}

func (f *TcpToWsForwarder) forwardWsToTcp(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := f.wsConn.ReadMessage()
		if err != nil {
			slog.Error("WebSocket 读取失败", "err", err)
			return
		}

		slog.Debug("WebSocket 收到数据", "data", hex.EncodeToString(message))

		_, err = f.conn.Write(message)
		if err != nil {
			slog.Error("TCP 写入失败", "err", err)
			return
		}
	}
}

func (f *TcpToWsForwarder) Close() {
	if f.conn != nil {
		f.conn.Close()
	}
}

type WsToTcpForwarder struct {
	wsConn    *websocket.Conn
	tcpConn   net.Conn
	mu        sync.Mutex
	wsUrl     string
	closeOnce sync.Once
}

func NewWsToTcpForwarder(wsUrl string, tcpConn net.Conn) *WsToTcpForwarder {
	return &WsToTcpForwarder{
		wsUrl:   wsUrl,
		tcpConn: tcpConn,
	}
}

func (f *WsToTcpForwarder) Connect(ctx context.Context) error {
	wsURL, err := url.Parse(f.wsUrl)
	if err != nil {
		return fmt.Errorf("解析 WebSocket URL 失败: %v", err)
	}

	slog.Info("开始建立 WebSocket 连接", "url", f.wsUrl)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	wsConn, _, err := dialer.Dial(f.wsUrl, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %v", err)
	}

	f.wsConn = wsConn
	slog.Info("WebSocket 连接成功", "host", wsURL.Host)

	return nil
}

func (f *WsToTcpForwarder) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		f.forwardTcpToWs(ctx)
	}()

	go func() {
		defer wg.Done()
		f.forwardWsToTcp(ctx)
	}()

	wg.Wait()
	f.Close()
}

func (f *WsToTcpForwarder) forwardTcpToWs(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := f.tcpConn.Read(buf)
		if err != nil {
			slog.Error("TCP 读取失败", "err", err)
			return
		}

		slog.Debug("TCP 收到数据", "data", hex.EncodeToString(buf[:n]))

		f.mu.Lock()
		err = f.wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
		f.mu.Unlock()

		if err != nil {
			slog.Error("WebSocket 写入失败", "err", err)
			return
		}
	}
}

func (f *WsToTcpForwarder) forwardWsToTcp(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := f.wsConn.ReadMessage()
		if err != nil {
			slog.Error("WebSocket 读取失败", "err", err)
			return
		}

		slog.Debug("WebSocket 收到数据", "data", hex.EncodeToString(message))

		_, err = f.tcpConn.Write(message)
		if err != nil {
			slog.Error("TCP 写入失败", "err", err)
			return
		}
	}
}

func (f *WsToTcpForwarder) Close() {
	f.closeOnce.Do(func() {
		if f.wsConn != nil {
			f.wsConn.Close()
		}
		if f.tcpConn != nil {
			f.tcpConn.Close()
		}
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	slog.Info("收到 WebSocket 连接请求", "remoteAddr", r.RemoteAddr)

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket 升级失败", "err", err)
		return
	}
	defer wsConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uri := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(uri, "/"), "/")

	if len(parts) < 4 || parts[1] != "forward" {
		slog.Warn("无效的路径", "uri", uri)
		return
	}

	targetHost := parts[2]
	targetPort := parts[3]

	slog.Info("WebSocket 连接建立，准备转发", "targetHost", targetHost, "targetPort", targetPort)

	forwarder := NewTcpToWsForwarder(targetHost, targetPort, wsConn)

	go func() {
		forwarder.Run(ctx)
		cancel()
	}()

	select {
	case <-ctx.Done():
		slog.Info("WebSocket 连接关闭")
	case <-r.Context().Done():
		slog.Info("请求上下文取消")
	}

	forwarder.Close()
}

func runServer(port int) {
	slog.Info("WebSocket Server 启动", "port", port)

	http.HandleFunc("/websocket/forward/", handleWebSocket)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr: ":" + strconv.Itoa(port),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务器启动失败", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("服务器正在关闭...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("服务器关闭失败", "err", err)
		os.Exit(1)
	}

	slog.Info("服务器已关闭")
}

func runClient(port int, wsUrl string) {
	addr := fmt.Sprintf(":%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("TCP 服务器启动失败", "err", err)
		os.Exit(1)
	}
	defer listener.Close()

	slog.Info("TCP Client 启动", "port", port, "wsUrl", wsUrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		slog.Info("收到关闭信号...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			slog.Info("TCP 服务器关闭")
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("接受连接失败", "err", err)
				continue
			}
		}

		slog.Info("收到 TCP 连接", "remoteAddr", conn.RemoteAddr())

		go func() {
			defer conn.Close()

			forwarder := NewWsToTcpForwarder(wsUrl, conn)

			if err := forwarder.Connect(ctx); err != nil {
				slog.Error("WebSocket 连接失败", "err", err)
				return
			}
			defer forwarder.Close()

			forwarder.Run(ctx)
			slog.Info("连接处理结束", "remoteAddr", conn.RemoteAddr())
		}()
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println(`TCP over WebSockets - 通过 WebSocket 转发 TCP 连接

用法:
    tcp_over_websockets <mode> <port> [wsUrl]

模式:
    server  - WebSocket 服务器模式，接收 WebSocket 连接并转发到目标 TCP
    client  - TCP 客户端模式，监听本地 TCP 端口并通过 WebSocket 转发

示例:
    # 启动 WebSocket 服务器，监听 7002 端口
    tcp_over_websockets server 7002

    # 启动客户端，本地监听 13306 端口，转发到 WebSocket 服务器
    tcp_over_websockets client 13306 ws://localhost:7002/websocket/forward/mysql.local/3306

使用场景:
    1. 先启动 server 模式在可访问目标服务的机器上
    2. 在本地启动 client 模式，通过 WebSocket 访问远程服务
    3. 本地应用连接 localhost:13306 即可访问远程 MySQL 服务
`)
		return
	}

	mode := os.Args[1]
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		slog.Error("无效的端口号", "port", os.Args[2], "err", err)
		os.Exit(1)
	}

	switch mode {
	case "server":
		runServer(port)
	case "client":
		if len(os.Args) < 4 {
			slog.Error("client 模式需要提供 WebSocket URL 参数")
			os.Exit(1)
		}
		wsUrl := os.Args[3]
		runClient(port, wsUrl)
	default:
		slog.Error("未知模式", "mode", mode)
		os.Exit(1)
	}
}
