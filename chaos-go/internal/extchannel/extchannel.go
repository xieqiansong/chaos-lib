// Package extchannel 实现「后端 → 浏览器扩展」的反向通道（方案 A：SSE 长连接）。
//
// 现状：扩展只通过 background.ts 的 HTTP 转发器把数据推给后端，后端从不主动触达扩展。
// 本包让后端可以反向下发指令：扩展用 EventSource 连上 /api/ext/stream 保持长连接，
// 后端通过 Broadcast 把指令推到这条已建立的连接；扩展执行后把结果 POST 回 /api/ext/response。
//
// 这是最小 Demo，未做任何鉴权，仅用于本地自托管环境验证闭环。
package extchannel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Command 是后端推送给扩展的指令。
// 为了能透传任意指令参数（如 bookmarks 的 nodeId/title/parentId/isFolder/query 等），
// 指令不绑定到固定结构体，而是以 map 形式整体保留，转发时拍平为顶层 JSON，
// 扩展侧即可直接按 cmd.nodeId 这样的字段读取，而不会丢字段。
type Command struct {
	Data map[string]any
}

// MarshalJSON 把指令拍平为顶层 JSON 对象（含 type、id 及所有透传参数）。
func (c Command) MarshalJSON() ([]byte, error) {
	if c.Data == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(c.Data)
}

// client 表示一条已连接的扩展 SSE 连接。
type client struct {
	ch chan Command
}

// ResponseRecord 是一条扩展回传记录，用于前端查看「后端 ↔ 扩展」的交换。
type ResponseRecord struct {
	ID         string `json:"id,omitempty"`
	Type       string `json:"type"`
	Ok         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	Echo       any    `json:"echo,omitempty"`
	ReceivedAt string `json:"receivedAt"`
}

var (
	mu      sync.Mutex
	clients = map[string]*client{}
	seq     int

	// 最近回传记录（环形，仅内存，进程重启清空）
	respMu      sync.Mutex
	responseLog = make([]ResponseRecord, 0, responseLogCap)

	// 阻塞等待器：push 指令后按指令 id 挂起，等扩展回传再唤醒（避免前端轮询）
	waitMu  sync.Mutex
	waiters = map[string]chan ResponseRecord{}
)

// pushTimeout 是 push 指令等待扩展回传的最长时间。
const pushTimeout = 12 * time.Second

const responseLogCap = 100

// register 注册一条新连接，返回其 client（调用方负责在结束时 unregister）。
func register() *client {
	mu.Lock()
	defer mu.Unlock()
	seq++
	id := fmt.Sprintf("ext-%d", seq)
	c := &client{ch: make(chan Command, 16)}
	clients[id] = c
	slog.Info("扩展反向通道：新连接", "id", id, "total", len(clients))
	return c
}

// unregister 从注册表移除连接。注意：不 close channel，避免并发 Broadcast 向已关闭
// channel 发送导致 panic；该 channel 会随 client 被 GC 回收。
func unregister(c *client) {
	mu.Lock()
	defer mu.Unlock()
	for k, v := range clients {
		if v == c {
			delete(clients, k)
			slog.Info("扩展反向通道：连接断开", "id", k, "total", len(clients))
			return
		}
	}
}

// Broadcast 把指令推送给所有已连接的扩展，返回成功送达的数量。
func Broadcast(cmd Command) int {
	mu.Lock()
	defer mu.Unlock()
	n := 0
	for _, c := range clients {
		select {
		case c.ch <- cmd:
			n++
		default:
			// 缓冲满（扩展消费不过来），跳过该客户端，不阻塞。
		}
	}
	return n
}

// Stream 是 SSE 端点：扩展连上后保持长连接，后端通过 Broadcast 推送指令。
func Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 关闭 nginx 缓冲，保证实时

	cl := register()
	defer unregister(cl)

	// 连接建立后先发一个 hello，便于扩展确认通道就绪。
	_ = writeEvent(c, Command{Data: map[string]any{"type": "hello"}})

	// 心跳：每 5 秒发一条 SSE 注释行（: 开头，扩展侧 onmessage 不会触发），
	// 让空闲连接持续有字节流动，避免被代理/网关/浏览器的空闲超时掐断。
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-cl.ch:
			if err := writeEvent(c, cmd); err != nil {
				return
			}
		case <-ticker.C:
			if err := writeHeartbeat(c); err != nil {
				return
			}
		case <-c.Request.Context().Done():
			// 客户端断开（含 MV3 service worker 被回收）
			return
		}
	}
}

// writeHeartbeat 写出一条 SSE 注释心跳（: 开头）。注释行会被 EventSource 忽略，
// 仅用于保活长连接。
func writeHeartbeat(c *gin.Context) error {
	if _, err := c.Writer.WriteString(": ping\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

// writeEvent 按 SSE 格式写出一条 data 事件。
func writeEvent(c *gin.Context, cmd Command) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	if _, err := c.Writer.WriteString("data: " + string(data) + "\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

// Push 是「后端主动下令」的触发入口（Demo 用）。真实场景可由业务 handler / scheduler 调用 Broadcast。
// 阻塞语义：广播后按指令 id 挂起，直到某个扩展回传结果或超时，结果随 HTTP 响应直接返回，前端无需轮询。
// 指令以 map 透传，所有字段（含 nodeId/title 等）都会原样发给扩展。
func Push(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if raw == nil {
		raw = map[string]any{}
	}
	t, _ := raw["type"].(string)
	if t == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 不能为空"})
		return
	}
	id, _ := raw["id"].(string)
	if id == "" {
		id = fmt.Sprintf("cmd-%d", time.Now().UnixNano())
		raw["id"] = id
	}

	cmd := Command{Data: raw}

	// 注册阻塞等待器（按指令 id 关联回传）
	ch := make(chan ResponseRecord, 1)
	waitMu.Lock()
	waiters[id] = ch
	waitMu.Unlock()
	defer func() {
		waitMu.Lock()
		delete(waiters, id)
		waitMu.Unlock()
	}()

	n := Broadcast(cmd)
	if n == 0 {
		// 没有已连接扩展：立即返回，不阻塞
		c.JSON(http.StatusOK, gin.H{"pushed": 0, "response": nil})
		return
	}

	select {
	case rec := <-ch:
		c.JSON(http.StatusOK, gin.H{"pushed": n, "response": rec})
	case <-time.After(pushTimeout):
		c.JSON(http.StatusOK, gin.H{
			"pushed": n,
			"response": ResponseRecord{
				ID:    id,
				Type:  t,
				Ok:    false,
				Error: "等待扩展回传超时",
			},
		})
	}
}

// Response 接收扩展执行指令后的回传结果：记录到内存日志，并唤醒对应的阻塞等待器。
func Response(c *gin.Context) {
	var in struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
		Echo  any    `json:"echo"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec := ResponseRecord{
		ID:         in.ID,
		Type:       in.Type,
		Ok:         in.Ok,
		Error:      in.Error,
		Echo:       in.Echo,
		ReceivedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	respMu.Lock()
	responseLog = append(responseLog, rec)
	if len(responseLog) > responseLogCap {
		responseLog = responseLog[len(responseLog)-responseLogCap:]
	}
	respMu.Unlock()

	// 唤醒阻塞中的 push 调用（按指令 id 匹配）
	waitMu.Lock()
	if ch, ok := waiters[in.ID]; ok {
		select {
		case ch <- rec:
		default:
		}
	}
	waitMu.Unlock()

	slog.Info("扩展反向通道：收到回传", "type", in.Type, "ok", in.Ok, "id", in.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Responses 返回最近的扩展回传记录（最新在前），用于前端展示「交换记录」。
func Responses(c *gin.Context) {
	respMu.Lock()
	out := make([]ResponseRecord, 0, len(responseLog))
	for i := len(responseLog) - 1; i >= 0; i-- {
		out = append(out, responseLog[i])
	}
	respMu.Unlock()
	c.JSON(http.StatusOK, out)
}

// Status 返回当前已连接的扩展数量，便于确认通道是否建立。
func Status(c *gin.Context) {
	mu.Lock()
	n := len(clients)
	mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"connected": n})
}
