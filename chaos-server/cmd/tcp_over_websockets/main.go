package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
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
	log.Printf("开始建立 TCP 连接 targetHost: %s targetPort: %s", f.targetHost, f.targetPort)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Printf("TCP 连接失败 %s: %v", addr, err)
		return
	}
	f.conn = conn
	defer conn.Close()

	log.Printf("TCP 连接成功 %s", addr)

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
	log.Printf("TCP 连接结束 %s", addr)
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
			log.Printf("TCP 读取失败: %v", err)
			return
		}

		log.Printf("TCP 收到数据: %s", hex.EncodeToString(buf[:n]))

		f.mu.Lock()
		err = f.wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
		f.mu.Unlock()

		if err != nil {
			log.Printf("WebSocket 写入失败: %v", err)
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
			log.Printf("WebSocket 读取失败: %v", err)
			return
		}

		log.Printf("WebSocket 收到数据: %s", hex.EncodeToString(message))

		_, err = f.conn.Write(message)
		if err != nil {
			log.Printf("TCP 写入失败: %v", err)
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

	log.Printf("开始建立 WebSocket 连接: %s", f.wsUrl)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	wsConn, _, err := dialer.Dial(f.wsUrl, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %v", err)
	}

	f.wsConn = wsConn
	log.Printf("WebSocket 连接成功: %s", wsURL.Host)

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
			log.Printf("TCP 读取失败: %v", err)
			return
		}

		log.Printf("TCP 收到数据: %s", hex.EncodeToString(buf[:n]))

		f.mu.Lock()
		err = f.wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
		f.mu.Unlock()

		if err != nil {
			log.Printf("WebSocket 写入失败: %v", err)
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
			log.Printf("WebSocket 读取失败: %v", err)
			return
		}

		log.Printf("WebSocket 收到数据: %s", hex.EncodeToString(message))

		_, err = f.tcpConn.Write(message)
		if err != nil {
			log.Printf("TCP 写入失败: %v", err)
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
	log.Printf("收到 WebSocket 连接请求: %s", r.RemoteAddr)

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer wsConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uri := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(uri, "/"), "/")

	if len(parts) < 4 || parts[1] != "forward" {
		log.Printf("无效的路径: %s", uri)
		return
	}

	targetHost := parts[2]
	targetPort := parts[3]

	log.Printf("WebSocket 连接建立，准备转发到 %s:%s", targetHost, targetPort)

	forwarder := NewTcpToWsForwarder(targetHost, targetPort, wsConn)

	go func() {
		forwarder.Run(ctx)
		cancel()
	}()

	select {
	case <-ctx.Done():
		log.Printf("WebSocket 连接关闭")
	case <-r.Context().Done():
		log.Printf("请求上下文取消")
	}

	forwarder.Close()
}

func runServer(port int) {
	log.Printf("WebSocket Server 启动在端口 %d", port)

	http.HandleFunc("/websocket/forward/", handleWebSocket)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr: ":" + strconv.Itoa(port),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("服务器正在关闭...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("服务器关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}

func runClient(port int, wsUrl string) {
	addr := fmt.Sprintf(":%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("TCP 服务器启动失败: %v", err)
	}
	defer listener.Close()

	log.Printf("TCP Client 启动在端口 %d，转发到 %s", port, wsUrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("收到关闭信号...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("TCP 服务器关闭")
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("接受连接失败: %v", err)
				continue
			}
		}

		log.Printf("收到 TCP 连接: %s", conn.RemoteAddr())

		go func() {
			defer conn.Close()

			forwarder := NewWsToTcpForwarder(wsUrl, conn)

			if err := forwarder.Connect(ctx); err != nil {
				log.Printf("WebSocket 连接失败: %v", err)
				return
			}
			defer forwarder.Close()

			forwarder.Run(ctx)
			log.Printf("连接处理结束: %s", conn.RemoteAddr())
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
		log.Fatalf("无效的端口号: %s", os.Args[2])
	}

	switch mode {
	case "server":
		runServer(port)
	case "client":
		if len(os.Args) < 4 {
			log.Fatal("client 模式需要提供 WebSocket URL 参数")
		}
		wsUrl := os.Args[3]
		runClient(port, wsUrl)
	default:
		log.Fatalf("未知模式: %s，请使用 'server' 或 'client'", mode)
	}
}
