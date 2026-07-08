package tools

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// PortForwarder 端口转发管理器
type PortForwarder struct {
	forwards map[int]*ForwardTask
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// ForwardTask 单个转发任务
type ForwardTask struct {
	localPort     int
	remoteAddr    string
	listener      net.Listener
	cancel        context.CancelFunc
	activeConns   sync.WaitGroup
	retryTicker   *time.Ticker
	stopRetry     chan struct{}
	activeConnsMu sync.Mutex
	connections   map[net.Conn]struct{}
}

// NewPortForwarder 创建新的端口转发管理器
func NewPortForwarder() *PortForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &PortForwarder{
		forwards: make(map[int]*ForwardTask),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// AddForward 添加端口转发（带自动重试）
func (pf *PortForwarder) AddForward(localPort int, remoteAddr string) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	// 检查是否已存在
	if _, exists := pf.forwards[localPort]; exists {
		log.Printf("端口 %d 已存在转发，跳过", localPort)
		return nil
	}

	// 创建监听
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return fmt.Errorf("监听端口 %d 失败: %v", localPort, err)
	}

	taskCtx, taskCancel := context.WithCancel(pf.ctx)

	task := &ForwardTask{
		localPort:   localPort,
		remoteAddr:  remoteAddr,
		listener:    listener,
		cancel:      taskCancel,
		stopRetry:   make(chan struct{}),
		retryTicker: time.NewTicker(15 * time.Second),
		connections: make(map[net.Conn]struct{}),
	}

	pf.forwards[localPort] = task

	// 启动重试监控器
	go pf.startRetryMonitor(task, taskCtx)

	// 启动转发服务
	go pf.startForwarding(task, taskCtx)

	log.Printf("✅ 已添加端口转发: :%d -> %s (自动重试已启用)", localPort, remoteAddr)
	return nil
}

// RemoveForward 删除端口转发（直接断开所有连接）
func (pf *PortForwarder) RemoveForward(localPort int) error {
	pf.mu.Lock()

	task, exists := pf.forwards[localPort]
	if !exists {
		pf.mu.Unlock()
		return nil
	}

	// 停止重试
	close(task.stopRetry)
	task.retryTicker.Stop()

	// 停止任务
	task.cancel()
	task.listener.Close()

	// 立即断开所有现有连接
	task.activeConnsMu.Lock()
	for conn := range task.connections {
		conn.Close()
	}
	task.activeConnsMu.Unlock()

	delete(pf.forwards, localPort)
	pf.mu.Unlock()

	log.Printf("🗑️ 端口转发 :%d 已停止，所有连接已断开", localPort)
	return nil
}

// startRetryMonitor 启动重试监控器
func (pf *PortForwarder) startRetryMonitor(task *ForwardTask, ctx context.Context) {
	defer task.retryTicker.Stop()

	// 初始立即尝试连接
	pf.tryConnectRemote(task, ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-task.stopRetry:
			return
		case <-task.retryTicker.C:
			pf.tryConnectRemote(task, ctx)
		}
	}
}

// tryConnectRemote 尝试连接远程服务
func (pf *PortForwarder) tryConnectRemote(task *ForwardTask, ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	conn, err := net.DialTimeout("tcp", task.remoteAddr, 5*time.Second)
	if err != nil {
		log.Printf("⚠️ 连接远程 %s 失败，15秒后重试: %v", task.remoteAddr, err)
		return
	}

	conn.Close()
	log.Printf("✅ 远程服务 %s 可达", task.remoteAddr)
}

// startForwarding 开始转发流量
func (pf *PortForwarder) startForwarding(task *ForwardTask, ctx context.Context) {
	defer task.listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := task.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("❌ 接受连接失败: %v", err)
				continue
			}
		}

		task.activeConns.Add(1)
		go pf.handleConnectionWithRetry(conn, task, &task.activeConns)
	}
}

// handleConnectionWithRetry 带重试的连接处理
func (pf *PortForwarder) handleConnectionWithRetry(localConn net.Conn, task *ForwardTask, wg *sync.WaitGroup) {
	defer localConn.Close()
	defer wg.Done()

	task.activeConnsMu.Lock()
	task.connections[localConn] = struct{}{}
	task.activeConnsMu.Unlock()

	defer func() {
		task.activeConnsMu.Lock()
		delete(task.connections, localConn)
		task.activeConnsMu.Unlock()
	}()

	remoteConn, err := pf.connectWithRetry(task.remoteAddr, 15*time.Second)
	if err != nil {
		log.Printf("❌ 无法连接远程 %s: %v", task.remoteAddr, err)
		return
	}
	defer remoteConn.Close()

	log.Printf("🔄 开始转发: %s <-> %s", localConn.RemoteAddr(), task.remoteAddr)

	// 双向转发
	done := make(chan struct{}, 2)

	// 本地 -> 远程
	go func() {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil && err != io.EOF {
			log.Printf("⚠️ 本地->远程转发中断: %v", err)
		}
		done <- struct{}{}
	}()

	// 远程 -> 本地
	go func() {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil && err != io.EOF {
			log.Printf("⚠️ 远程->本地转发中断: %v", err)
		}
		done <- struct{}{}
	}()

	// 等待任意一端断开
	<-done
	log.Printf("🔌 连接关闭: %s", localConn.RemoteAddr())
}

// connectWithRetry 带重试的远程连接
func (pf *PortForwarder) connectWithRetry(remoteAddr string, retryInterval time.Duration) (net.Conn, error) {
	var lastErr error

	for {
		select {
		case <-pf.ctx.Done():
			return nil, fmt.Errorf("转发器已停止")
		default:
		}

		conn, err := net.DialTimeout("tcp", remoteAddr, 5*time.Second)
		if err == nil {
			return conn, nil
		}

		lastErr = err
		log.Printf("🔄 连接 %s 失败，%v 后重试: %v", remoteAddr, retryInterval, err)

		select {
		case <-time.After(retryInterval):
			continue
		case <-pf.ctx.Done():
			return nil, lastErr
		}
	}
}

// ListForwards 列出所有转发
func (pf *PortForwarder) ListForwards() {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	fmt.Println("\n📋 活跃的端口转发:")
	if len(pf.forwards) == 0 {
		fmt.Println("  (空)")
		return
	}

	for port, task := range pf.forwards {
		fmt.Printf("  :%d -> %s 🔄\n", port, task.remoteAddr)
	}
}

// StopAll 停止所有转发
func (pf *PortForwarder) StopAll() {
	pf.cancel()
	pf.mu.Lock()
	defer pf.mu.Unlock()

	for _, task := range pf.forwards {
		close(task.stopRetry)
		task.retryTicker.Stop()
		task.listener.Close()
		task.cancel()
	}
	pf.forwards = make(map[int]*ForwardTask)
}

var GlobalPortForwarder = NewPortForwarder()

// ListForwardsInfo 返回所有转发的信息列表
func (pf *PortForwarder) ListForwardsInfo() []struct {
	LocalPort  int    ``
	RemoteAddr string ``
} {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	result := make([]struct {
		LocalPort  int    ``
		RemoteAddr string ``
	}, 0, len(pf.forwards))

	for port, task := range pf.forwards {
		result = append(result, struct {
			LocalPort  int    ``
			RemoteAddr string ``
		}{port, task.remoteAddr})
	}

	return result
}
