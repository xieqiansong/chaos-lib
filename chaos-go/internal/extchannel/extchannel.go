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
type Command struct {
	ID   string `json:"id,omitempty"`   // 指令唯一 id，扩展回传时原样带回，便于对账
	Type string `json:"type"`           // 指令类型：openTab / ping / ...
	URL  string `json:"url,omitempty"`  // openTab 用
	Text string `json:"text,omitempty"` // 可选附带文本
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
)

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
	_ = writeEvent(c, Command{Type: "hello"})

	for {
		select {
		case cmd := <-cl.ch:
			if err := writeEvent(c, cmd); err != nil {
				return
			}
		case <-c.Request.Context().Done():
			// 客户端断开（含 MV3 service worker 被回收）
			return
		}
	}
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
func Push(c *gin.Context) {
	var cmd Command
	if err := c.ShouldBindJSON(&cmd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cmd.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 不能为空"})
		return
	}
	n := Broadcast(cmd)
	c.JSON(http.StatusOK, gin.H{"pushed": n})
}

// Response 接收扩展执行指令后的回传结果，记录到内存日志供前端查看交换记录。
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
