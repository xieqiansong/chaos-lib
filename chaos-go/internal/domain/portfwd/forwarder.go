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

	"golang.org/x/crypto/ssh"
)

const (
	connectRetry = 15 * time.Second

	keepAlivePeriod = 30 * time.Second

	dialRetryInterval = time.Second

	localDialTimeout = 5 * time.Second
)

var errTunnelClosed = errors.New("隧道已停止")

type PortForwarder struct {
	forwards map[int]*ForwardTask
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

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

func (pf *PortForwarder) AddForward(rule *PortForwarding, sshConn *SshConnection) error {
	if err := rule.normalize(); err != nil {
		return err
	}

	pf.mu.RLock()
	_, exists := pf.forwards[rule.ID]
	pf.mu.RUnlock()
	if exists {
		return fmt.Errorf("该端口转发已启动")
	}

	if isLocalSideListen(rule.Direction) {
		pf.mu.RLock()
		for _, other := range pf.forwards {
			if isLocalSideListen(other.direction) && other.listenAddr == rule.listenAddr() {
				pf.mu.RUnlock()
				return fmt.Errorf("本机端口 %d 已在其他转发中使用", rule.Port)
			}
		}
		pf.mu.RUnlock()
	}

	var client *ssh.Client
	if rule.Direction != DirectionDirect {
		if sshConn == nil {
			return fmt.Errorf("请选择 SSH 连接")
		}
		var err error
		client, err = sshConn.dial()
		if err != nil {
			return fmt.Errorf("SSH 连接失败: %v", err)
		}
	}

	taskCtx, taskCancel := context.WithCancel(pf.ctx)
	task := &ForwardTask{
		ruleId: rule.ID, direction: rule.Direction, port: rule.Port,
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
			if client != nil {
				client.Close()
			}
			return fmt.Errorf("在 SSH 服务器侧监听 %s 失败: %v", task.listenAddr, err)
		}
		task.setListener(listener)
	} else {

		listener, err := net.Listen("tcp", task.listenAddr)
		if err != nil {
			taskCancel()
			if client != nil {
				client.Close()
			}
			return fmt.Errorf("监听本机端口 %d 失败: %v", rule.Port, err)
		}
		task.setListener(listener)
	}

	pf.mu.Lock()
	if _, dup := pf.forwards[rule.ID]; dup {
		pf.mu.Unlock()
		taskCancel()
		task.closeListener()
		if client != nil {
			client.Close()
		}
		return fmt.Errorf("该端口转发已启动")
	}
	pf.forwards[rule.ID] = task
	pf.mu.Unlock()

	if rule.Direction == DirectionRemote {
		go pf.remoteListenLoop(task)
	} else {
		go pf.localAcceptLoop(task)
	}

	if rule.Direction != DirectionDirect {
		go pf.keepAliveLoop(task)
	}

	sshAddr := ""
	if sshConn != nil {
		sshAddr = sshConn.sshAddr()
	}
	slog.Info("端口转发已启动",
		"ruleId", rule.ID, "direction", rule.Direction, "listen", task.listenAddr,
		"sshAddr", sshAddr, "target", task.targetAddr)
	return nil
}

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

	slog.Info("端口转发已停止", "ruleId", ruleId, "direction", task.direction, "listen", task.listenAddr)
	return nil
}

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
		sshAddr := ""
		if task.sshConn != nil {
			sshAddr = task.sshConn.sshAddr()
		}
		statuses = append(statuses, ForwardStatus{
			RuleId: task.ruleId, Direction: task.direction, Port: task.port,
			Listen: task.listenAddr, Target: task.targetAddr,
			SshAddr: sshAddr, LastError: task.lastErr,
		})
		task.clientMu.Unlock()
	}
	return statuses
}

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

func RecoverForwardsOnBoot() {
	rules, err := FindEnabledPortForwardings()
	if err != nil {
		slog.Warn("读取待恢复端口转发失败", "err", err)
		return
	}
	if len(rules) == 0 {
		return
	}
	slog.Info("启动恢复端口转发", "count", len(rules))
	for i := range rules {
		rule := &rules[i]
		_ = rule.normalize()
		var conn *SshConnection
		if rule.Direction != DirectionDirect {
			loaded, lerr := FindSshConnByID(rule.SshConnectionId)
			if lerr != nil {
				slog.Warn("端口转发恢复跳过：SSH 连接不存在",
					"ruleId", rule.ID, "name", rule.Name, "err", lerr)
				continue
			}
			conn = loaded
		}
		if err := GlobalPortForwarder.AddForward(rule, conn); err != nil {
			slog.Warn("端口转发恢复失败（后续自愈任务会重试）",
				"ruleId", rule.ID, "name", rule.Name, "err", err)
		} else {
			slog.Info("端口转发已恢复", "ruleId", rule.ID, "name", rule.Name)
		}
	}
}

func SelfHealForwards() {
	rules, err := FindEnabledPortForwardings()
	if err != nil {
		slog.Warn("端口转发自愈扫描失败", "err", err)
		return
	}
	for i := range rules {
		rule := &rules[i]
		if running, _ := GlobalPortForwarder.Status(rule.ID); running {
			continue
		}
		_ = rule.normalize()
		var conn *SshConnection
		if rule.Direction != DirectionDirect {
			loaded, lerr := FindSshConnByID(rule.SshConnectionId)
			if lerr != nil {
				continue
			}
			conn = loaded
		}
		if err := GlobalPortForwarder.AddForward(rule, conn); err != nil {
			slog.Debug("端口转发自愈重连失败",
				"ruleId", rule.ID, "name", rule.Name, "err", err)
		} else {
			slog.Info("端口转发自愈重连成功", "ruleId", rule.ID, "name", rule.Name)
		}
	}
}

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

func (task *ForwardTask) dialOtherSide() (net.Conn, error) {
	if task.direction == DirectionLocal {
		return task.dialTargetViaSSH()
	}
	return task.dialDirectTarget()
}

func (task *ForwardTask) dialDirectTarget() (net.Conn, error) {
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

		task.closeListener()
	}
	if old != nil {
		old.Close()
	}
	task.setLastErr("")
	slog.Info("SSH 隧道已重连", "ruleId", task.ruleId, "direction", task.direction)
}

func isAuthError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "解析私钥失败") ||
		strings.Contains(msg, "未配置密码") ||
		strings.Contains(msg, "未配置私钥")
}

var GlobalPortForwarder = NewPortForwarder()
