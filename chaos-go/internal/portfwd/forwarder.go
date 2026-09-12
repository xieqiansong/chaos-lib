package portfwd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"chaos-go/config"

	"golang.org/x/crypto/ssh"
)

const (
	// connectRetry 单条本地连接等待隧道就绪的上限
	connectRetry = 15 * time.Second
	// keepAlivePeriod SSH 隧道保活探测间隔
	keepAlivePeriod = 30 * time.Second
	// dialRetryInterval 远端通道打开失败后的重试间隔
	dialRetryInterval = time.Second
)

var errTunnelClosed = errors.New("隧道已停止")

// PortForwarder 管理所有 SSH 隧道转发任务，进程内单例（GlobalPortForwarder）。
type PortForwarder struct {
	forwards map[int]*ForwardTask
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// ForwardTask 一条转发规则的运行时状态：1 个本地 listener + 1 个 SSH 客户端。
type ForwardTask struct {
	localPort  int
	targetAddr string
	sshConn    *SshConnection
	owner      *PortForwarder

	ctx    context.Context
	cancel context.CancelFunc

	listener net.Listener

	clientMu     sync.Mutex
	client       *ssh.Client
	lastErr      string
	reconnectOff bool

	connsMu     sync.Mutex
	connections map[net.Conn]struct{}
}

// ForwardStatus 运行中转发任务的快照。
type ForwardStatus struct {
	LocalPort int
	Target    string
	SshAddr   string
	LastError string
}

func NewPortForwarder() *PortForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &PortForwarder{forwards: make(map[int]*ForwardTask), ctx: ctx, cancel: cancel}
}

// AddForward 建立 SSH 隧道并监听本地端口。
// 先完成 SSH 认证再监听端口：认证失败时不占用本地端口。
func (pf *PortForwarder) AddForward(rule *PortForwarding, sshConn *SshConnection) error {
	pf.mu.RLock()
	_, exists := pf.forwards[rule.Port]
	pf.mu.RUnlock()
	if exists {
		return fmt.Errorf("本地端口 %d 已在转发中", rule.Port)
	}

	client, err := sshConn.dial()
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %v", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", rule.Port))
	if err != nil {
		client.Close()
		return fmt.Errorf("监听本地端口 %d 失败: %v", rule.Port, err)
	}

	taskCtx, taskCancel := context.WithCancel(pf.ctx)
	task := &ForwardTask{
		localPort: rule.Port, targetAddr: rule.targetAddr(),
		sshConn: sshConn, owner: pf,
		ctx: taskCtx, cancel: taskCancel,
		listener: listener, client: client,
		connections: make(map[net.Conn]struct{}),
	}

	pf.mu.Lock()
	if _, dup := pf.forwards[rule.Port]; dup {
		pf.mu.Unlock()
		taskCancel()
		listener.Close()
		client.Close()
		return fmt.Errorf("本地端口 %d 已在转发中", rule.Port)
	}
	pf.forwards[rule.Port] = task
	pf.mu.Unlock()

	go pf.acceptLoop(task)
	go pf.keepAliveLoop(task)
	slog.Info("SSH 端口转发已启动",
		"localPort", rule.Port, "sshAddr", sshConn.sshAddr(), "target", task.targetAddr)
	return nil
}

// RemoveForward 停止转发：关闭 listener → 关闭活动连接 → 关闭 SSH 客户端。
func (pf *PortForwarder) RemoveForward(localPort int) error {
	pf.mu.Lock()
	task, exists := pf.forwards[localPort]
	if exists {
		delete(pf.forwards, localPort)
	}
	pf.mu.Unlock()
	if !exists {
		return nil
	}

	task.cancel()
	task.listener.Close()
	task.closeClient()

	task.connsMu.Lock()
	for conn := range task.connections {
		conn.Close()
	}
	task.connsMu.Unlock()

	slog.Info("SSH 端口转发已停止", "localPort", localPort)
	return nil
}

// Status 返回本地端口是否正在转发，以及最近一次错误。
func (pf *PortForwarder) Status(localPort int) (bool, string) {
	pf.mu.RLock()
	task, exists := pf.forwards[localPort]
	pf.mu.RUnlock()
	if !exists {
		return false, ""
	}
	task.clientMu.Lock()
	defer task.clientMu.Unlock()
	return true, task.lastErr
}

// ListForwards 返回当前所有运行中的转发任务。
func (pf *PortForwarder) ListForwards() []ForwardStatus {
	pf.mu.RLock()
	tasks := make([]*ForwardTask, 0, len(pf.forwards))
	for _, task := range pf.forwards {
		tasks = append(tasks, task)
	}
	pf.mu.RUnlock()

	statuses := make([]ForwardStatus, 0, len(tasks))
	for _, task := range tasks {
		task.clientMu.Lock()
		statuses = append(statuses, ForwardStatus{
			LocalPort: task.localPort, Target: task.targetAddr,
			SshAddr: task.sshConn.sshAddr(), LastError: task.lastErr,
		})
		task.clientMu.Unlock()
	}
	return statuses
}

// StopAll 停止全部转发（进程退出时调用）。
func (pf *PortForwarder) StopAll() {
	pf.cancel()
	pf.mu.Lock()
	tasks := make([]*ForwardTask, 0, len(pf.forwards))
	for _, task := range pf.forwards {
		tasks = append(tasks, task)
	}
	pf.forwards = make(map[int]*ForwardTask)
	pf.mu.Unlock()

	for _, task := range tasks {
		task.listener.Close()
		task.closeClient()
		task.connsMu.Lock()
		for conn := range task.connections {
			conn.Close()
		}
		task.connsMu.Unlock()
	}
}

// ResetStatusOnBoot 进程启动时把库里遗留的「运行中」状态归零：
// 重启后内存中没有任何隧道，展示状态必须与内存一致。
func ResetStatusOnBoot() {
	db := config.GetDB()
	if db == nil {
		return
	}
	if err := db.Model(&PortForwarding{}).Where("status = ?", true).Update("status", false).Error; err != nil {
		slog.Warn("重置端口转发状态失败", "err", err)
	}
}

// ── 内部实现 ──────────────────────────────────────────────────────

func (task *ForwardTask) track(conn net.Conn) {
	task.connsMu.Lock()
	task.connections[conn] = struct{}{}
	task.connsMu.Unlock()
}

func (task *ForwardTask) untrack(conn net.Conn) {
	task.connsMu.Lock()
	delete(task.connections, conn)
	task.connsMu.Unlock()
}

func (task *ForwardTask) setLastErr(msg string) {
	task.clientMu.Lock()
	task.lastErr = msg
	task.clientMu.Unlock()
}

func (task *ForwardTask) getClient() *ssh.Client {
	task.clientMu.Lock()
	defer task.clientMu.Unlock()
	return task.client
}

func (task *ForwardTask) closeClient() {
	task.clientMu.Lock()
	client := task.client
	task.client = nil
	task.clientMu.Unlock()
	if client != nil {
		client.Close()
	}
}

func (pf *PortForwarder) acceptLoop(task *ForwardTask) {
	for {
		conn, err := task.listener.Accept()
		if err != nil {
			select {
			case <-task.ctx.Done():
				return
			default:
			}
			slog.Warn("接受本地连接失败", "localPort", task.localPort, "err", err)
			time.Sleep(dialRetryInterval)
			continue
		}
		task.track(conn)
		go pf.handleConnection(task, conn)
	}
}

// handleConnection 为本条本地连接打开一个 SSH direct-tcpip 通道并做双向转发。
func (pf *PortForwarder) handleConnection(task *ForwardTask, localConn net.Conn) {
	defer func() {
		task.untrack(localConn)
		localConn.Close()
	}()

	remoteConn, err := task.dialTarget()
	if err != nil {
		task.setLastErr(err.Error())
		slog.Warn("打开远端通道失败",
			"localPort", task.localPort, "target", task.targetAddr, "err", err)
		return
	}
	defer remoteConn.Close()

	task.setLastErr("")
	slog.Info("开始转发", "localPort", task.localPort, "target", task.targetAddr)

	done := make(chan struct{}, 2)
	go func() {
		if _, err := io.Copy(remoteConn, localConn); err != nil && err != io.EOF {
			slog.Warn("本地→远端转发中断", "localPort", task.localPort, "err", err)
		}
		done <- struct{}{}
	}()
	go func() {
		if _, err := io.Copy(localConn, remoteConn); err != nil && err != io.EOF {
			slog.Warn("远端→本地转发中断", "localPort", task.localPort, "err", err)
		}
		done <- struct{}{}
	}()
	<-done
}

// dialTarget 通过 SSH 通道连接远端目标；隧道未就绪时在 connectRetry 内重试。
func (task *ForwardTask) dialTarget() (net.Conn, error) {
	deadline := time.Now().Add(connectRetry)
	lastErr := error(errTunnelClosed)
	for {
		select {
		case <-task.ctx.Done():
			return nil, errTunnelClosed
		default:
		}

		if client := task.getClient(); client != nil {
			conn, err := client.Dial("tcp", task.targetAddr)
			if err == nil {
				return conn, nil
			}
			lastErr = err
			// 通道打开失败通常意味着隧道已断，立刻尝试重连一次
			task.owner.reconnect(task)
		}

		if time.Now().After(deadline) {
			return nil, lastErr
		}
		select {
		case <-time.After(dialRetryInterval):
		case <-task.ctx.Done():
			return nil, errTunnelClosed
		}
	}
}

func (pf *PortForwarder) keepAliveLoop(task *ForwardTask) {
	ticker := time.NewTicker(keepAlivePeriod)
	defer ticker.Stop()
	for {
		select {
		case <-task.ctx.Done():
			return
		case <-ticker.C:
		}

		task.clientMu.Lock()
		client := task.client
		stopped := task.reconnectOff
		task.clientMu.Unlock()
		if stopped {
			return
		}
		if client == nil {
			continue
		}
		if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			slog.Warn("SSH 隧道保活失败，尝试重连", "localPort", task.localPort, "err", err)
			pf.reconnect(task)
		}
	}
}

// reconnect 重建 SSH 客户端；认证类错误不重试（避免反复失败刷日志）。
func (pf *PortForwarder) reconnect(task *ForwardTask) {
	task.clientMu.Lock()
	if task.reconnectOff || task.client == nil {
		task.clientMu.Unlock()
		return
	}
	task.clientMu.Unlock()

	client, err := task.sshConn.dial()
	if err != nil {
		task.setLastErr(err.Error())
		if isAuthError(err) {
			task.clientMu.Lock()
			task.reconnectOff = true
			task.clientMu.Unlock()
			slog.Error("SSH 重新认证失败，已停止自动重连",
				"localPort", task.localPort, "sshAddr", task.sshConn.sshAddr(), "err", err)
			return
		}
		slog.Warn("SSH 隧道重连失败，稍后重试", "localPort", task.localPort, "err", err)
		return
	}

	task.clientMu.Lock()
	old := task.client
	task.client = client
	task.reconnectOff = false
	task.clientMu.Unlock()
	if old != nil {
		old.Close()
	}
	task.setLastErr("")
	slog.Info("SSH 隧道已重连", "localPort", task.localPort)
}

// isAuthError 判断是否为认证/凭据类错误（不重试），区别于网络类错误（可重试）。
func isAuthError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "解析私钥失败") ||
		strings.Contains(msg, "未配置密码") ||
		strings.Contains(msg, "未配置私钥")
}

var GlobalPortForwarder = NewPortForwarder()
