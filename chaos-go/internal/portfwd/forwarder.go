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
	// connectRetry 单条接入连接等待对端就绪的上限
	connectRetry = 15 * time.Second
	// keepAlivePeriod SSH 隧道保活探测间隔
	keepAlivePeriod = 30 * time.Second
	// dialRetryInterval 失败后的重试间隔
	dialRetryInterval = time.Second
	// localDialTimeout 远程转发时本机连接目标服务的超时
	localDialTimeout = 5 * time.Second
)

var errTunnelClosed = errors.New("隧道已停止")

// PortForwarder 管理所有 SSH 隧道转发任务，进程内单例（GlobalPortForwarder）。
// 任务以规则 id 为键：本地转发与远程转发可能使用相同端口号（分处本机与服务器侧），端口不能作为唯一键。
type PortForwarder struct {
	forwards map[int]*ForwardTask
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// ForwardTask 一条转发规则的运行时状态。
// 监听器来源随方向不同：local 用本机 net.Listen；remote 用 ssh.Client.Listen（位于 SSH 服务器侧，随客户端生命周期失效）。
type ForwardTask struct {
	ruleId     int
	direction  string
	port       int
	bindAddr   string
	listenAddr string
	targetAddr string
	sshConn    *SshConnection
	owner      *PortForwarder

	ctx    context.Context
	cancel context.CancelFunc

	listenerMu sync.Mutex
	listener   net.Listener

	clientMu     sync.Mutex
	client       *ssh.Client
	lastErr      string
	reconnectOff bool

	connsMu     sync.Mutex
	connections map[net.Conn]struct{}
}

// ForwardStatus 运行中转发任务的快照。
type ForwardStatus struct {
	RuleId    int
	Direction string
	Port      int
	Listen    string
	Target    string
	SshAddr   string
	LastError string
}

func NewPortForwarder() *PortForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &PortForwarder{forwards: make(map[int]*ForwardTask), ctx: ctx, cancel: cancel}
}

// AddForward 建立 SSH 隧道并按方向完成监听。
// 先完成 SSH 认证再监听：认证失败或监听失败都不留下半启动状态。
func (pf *PortForwarder) AddForward(rule *PortForwarding, sshConn *SshConnection) error {
	if err := rule.normalize(); err != nil {
		return err
	}

	pf.mu.RLock()
	_, exists := pf.forwards[rule.Id]
	pf.mu.RUnlock()
	if exists {
		return fmt.Errorf("该端口转发已启动")
	}
	// 本机监听端口由本机独占：同一 (方向, 监听地址) 已被其他规则占用时提前给出明确错误
	if rule.Direction == DirectionLocal {
		pf.mu.RLock()
		for _, other := range pf.forwards {
			if other.direction == DirectionLocal && other.listenAddr == rule.listenAddr() {
				pf.mu.RUnlock()
				return fmt.Errorf("本机端口 %d 已在其他转发中使用", rule.Port)
			}
		}
		pf.mu.RUnlock()
	}

	client, err := sshConn.dial()
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %v", err)
	}

	taskCtx, taskCancel := context.WithCancel(pf.ctx)
	task := &ForwardTask{
		ruleId: rule.Id, direction: rule.Direction, port: rule.Port,
		bindAddr: rule.BindAddress, listenAddr: rule.listenAddr(), targetAddr: rule.targetAddr(),
		sshConn: sshConn, owner: pf,
		ctx: taskCtx, cancel: taskCancel,
		client:      client,
		connections: make(map[net.Conn]struct{}),
	}

	if rule.Direction == DirectionRemote {
		listener, err := client.Listen("tcp", task.listenAddr)
		if err != nil {
			taskCancel()
			client.Close()
			return fmt.Errorf("在 SSH 服务器侧监听 %s 失败: %v", task.listenAddr, err)
		}
		task.setListener(listener)
	} else {
		listener, err := net.Listen("tcp", task.listenAddr)
		if err != nil {
			taskCancel()
			client.Close()
			return fmt.Errorf("监听本机端口 %d 失败: %v", rule.Port, err)
		}
		task.setListener(listener)
	}

	pf.mu.Lock()
	if _, dup := pf.forwards[rule.Id]; dup {
		pf.mu.Unlock()
		taskCancel()
		task.closeListener()
		client.Close()
		return fmt.Errorf("该端口转发已启动")
	}
	pf.forwards[rule.Id] = task
	pf.mu.Unlock()

	if rule.Direction == DirectionRemote {
		go pf.remoteListenLoop(task)
	} else {
		go pf.localAcceptLoop(task)
	}
	go pf.keepAliveLoop(task)

	slog.Info("SSH 端口转发已启动",
		"ruleId", rule.Id, "direction", rule.Direction, "listen", task.listenAddr,
		"sshAddr", sshConn.sshAddr(), "target", task.targetAddr)
	return nil
}

// RemoveForward 停止转发：取消上下文 → 关闭监听 → 关闭活动连接 → 关闭 SSH 客户端。
func (pf *PortForwarder) RemoveForward(ruleId int) error {
	pf.mu.Lock()
	task, exists := pf.forwards[ruleId]
	if exists {
		delete(pf.forwards, ruleId)
	}
	pf.mu.Unlock()
	if !exists {
		return nil
	}

	task.cancel()
	task.closeListener()
	task.closeClient()

	task.connsMu.Lock()
	for conn := range task.connections {
		conn.Close()
	}
	task.connsMu.Unlock()

	slog.Info("SSH 端口转发已停止", "ruleId", ruleId, "direction", task.direction, "listen", task.listenAddr)
	return nil
}

// Status 返回该规则是否正在转发，以及最近一次错误。
func (pf *PortForwarder) Status(ruleId int) (bool, string) {
	pf.mu.RLock()
	task, exists := pf.forwards[ruleId]
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
			RuleId: task.ruleId, Direction: task.direction, Port: task.port,
			Listen: task.listenAddr, Target: task.targetAddr,
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
		task.closeListener()
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

func (task *ForwardTask) setListener(listener net.Listener) {
	task.listenerMu.Lock()
	task.listener = listener
	task.listenerMu.Unlock()
}

func (task *ForwardTask) getListener() net.Listener {
	task.listenerMu.Lock()
	defer task.listenerMu.Unlock()
	return task.listener
}

func (task *ForwardTask) closeListener() {
	task.listenerMu.Lock()
	listener := task.listener
	task.listener = nil
	task.listenerMu.Unlock()
	if listener != nil {
		listener.Close()
	}
}

func (task *ForwardTask) stopped() bool {
	return task.ctx.Err() != nil
}

// localAcceptLoop 本机监听：listener 一次创建、跨重连存活，因此这里持续 accept。
func (pf *PortForwarder) localAcceptLoop(task *ForwardTask) {
	listener := task.getListener()
	if listener == nil {
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			if task.stopped() {
				return
			}
			slog.Warn("接受本机连接失败", "ruleId", task.ruleId, "listen", task.listenAddr, "err", err)
			time.Sleep(dialRetryInterval)
			continue
		}
		task.track(conn)
		go pf.handleConnection(task, conn)
	}
}

// remoteListenLoop 服务器侧监听：listener 与 SSH 客户端生命周期绑定，
// 一旦失效（断线或客户端重建）就在当前客户端上重新申请监听，保证重连后自动恢复。
func (pf *PortForwarder) remoteListenLoop(task *ForwardTask) {
	for {
		if task.stopped() {
			return
		}
		client := task.getClient()
		if client == nil {
			time.Sleep(dialRetryInterval)
			continue
		}
		listener, err := client.Listen("tcp", task.listenAddr)
		if err != nil {
			task.setLastErr(fmt.Sprintf("在 SSH 服务器侧监听 %s 失败: %v", task.listenAddr, err))
			if isAuthError(err) {
				task.clientMu.Lock()
				task.reconnectOff = true
				task.clientMu.Unlock()
				slog.Error("SSH 远程监听认证失败，已停止自动重试",
					"ruleId", task.ruleId, "listen", task.listenAddr, "err", err)
				return
			}
			slog.Warn("SSH 远程监听失败，稍后重试",
				"ruleId", task.ruleId, "listen", task.listenAddr, "err", err)
			// 监听失败往往意味着当前客户端已不可用，主动换一条连接
			pf.reconnect(task)
			time.Sleep(dialRetryInterval)
			continue
		}

		task.setListener(listener)
		task.setLastErr("")
		slog.Info("SSH 远程监听已建立", "ruleId", task.ruleId, "listen", task.listenAddr)

		pf.serveRemoteListener(task, listener)
		task.setListener(nil)

		if task.stopped() {
			return
		}
		slog.Warn("SSH 远程监听中断，准备重建", "ruleId", task.ruleId, "listen", task.listenAddr)
		time.Sleep(dialRetryInterval)
	}
}

// serveRemoteListener 在给定 listener 上 accept，直到该 listener 失效。
func (pf *PortForwarder) serveRemoteListener(task *ForwardTask, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		task.track(conn)
		go pf.handleConnection(task, conn)
	}
}

// handleConnection 把接入连接与对端连接对接后双向转发。
func (pf *PortForwarder) handleConnection(task *ForwardTask, listenConn net.Conn) {
	defer func() {
		task.untrack(listenConn)
		listenConn.Close()
	}()

	otherConn, err := task.dialOtherSide()
	if err != nil {
		task.setLastErr(err.Error())
		slog.Warn("建立对端连接失败",
			"ruleId", task.ruleId, "direction", task.direction, "target", task.targetAddr, "err", err)
		return
	}
	defer otherConn.Close()

	task.setLastErr("")
	slog.Info("开始转发",
		"ruleId", task.ruleId, "direction", task.direction,
		"listen", task.listenAddr, "target", task.targetAddr)

	done := make(chan struct{}, 2)
	go func() {
		if _, err := io.Copy(otherConn, listenConn); err != nil && err != io.EOF {
			slog.Warn("转发中断（接入→对端）", "ruleId", task.ruleId, "err", err)
		}
		done <- struct{}{}
	}()
	go func() {
		if _, err := io.Copy(listenConn, otherConn); err != nil && err != io.EOF {
			slog.Warn("转发中断（对端→接入）", "ruleId", task.ruleId, "err", err)
		}
		done <- struct{}{}
	}()
	<-done
}

// dialOtherSide 按方向选择对端连接方式：
// local —— 远端目标由 SSH 服务器侧解析（等价 ssh -L）；
// remote —— 目标在本机侧解析（等价 ssh -R）。
func (task *ForwardTask) dialOtherSide() (net.Conn, error) {
	if task.direction == DirectionRemote {
		return task.dialLocalTarget()
	}
	return task.dialTargetViaSSH()
}

// dialLocalTarget 远程转发的对端：本机直连目标服务（目标地址在本机解析）。
func (task *ForwardTask) dialLocalTarget() (net.Conn, error) {
	deadline := time.Now().Add(connectRetry)
	lastErr := error(errTunnelClosed)
	for {
		if task.stopped() {
			return nil, errTunnelClosed
		}
		conn, err := net.DialTimeout("tcp", task.targetAddr, localDialTimeout)
		if err == nil {
			return conn, nil
		}
		lastErr = err
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

// dialTargetViaSSH 本地转发的对端：经 SSH 通道打开 direct-tcpip（目标地址在服务器侧解析）。
func (task *ForwardTask) dialTargetViaSSH() (net.Conn, error) {
	deadline := time.Now().Add(connectRetry)
	lastErr := error(errTunnelClosed)
	for {
		if task.stopped() {
			return nil, errTunnelClosed
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
			slog.Warn("SSH 隧道保活失败，尝试重连", "ruleId", task.ruleId, "err", err)
			pf.reconnect(task)
		}
	}
}

// reconnect 重建 SSH 客户端；认证类错误不重试（避免反复失败刷日志）。
// 远程转发的监听器与客户端绑定，因此换客户端后需要让监听循环重建它。
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
				"ruleId", task.ruleId, "sshAddr", task.sshConn.sshAddr(), "err", err)
			return
		}
		slog.Warn("SSH 隧道重连失败，稍后重试", "ruleId", task.ruleId, "err", err)
		return
	}

	task.clientMu.Lock()
	old := task.client
	task.client = client
	task.reconnectOff = false
	task.clientMu.Unlock()

	if task.direction == DirectionRemote {
		// 旧监听器绑在旧客户端上已失效，关掉它让 remoteListenLoop 用新客户端重新 Listen
		task.closeListener()
	}
	if old != nil {
		old.Close()
	}
	task.setLastErr("")
	slog.Info("SSH 隧道已重连", "ruleId", task.ruleId, "direction", task.direction)
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
