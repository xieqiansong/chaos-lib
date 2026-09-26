package stunpf

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaos-go/config"
	"chaos-go/internal/mqttsync"
	"chaos-go/internal/portfwd"
)

// stunTopicPrefix 是 STUN 消息 channel 的前缀，与 stunsync 发布时一致。
const stunTopicPrefix = "stun/"

// stunPayload 同时兼容两种报文体：
//   - 新格式：已拆好的 PublicHost / PublicPort；
//   - 旧格式：仅 PublicAddr（host:port），由本任务再拆分。
type stunPayload struct {
	Name       string `json:"Name"`
	TargetHost string `json:"TargetHost"`
	TargetPort string `json:"TargetPort"`
	PublicHost string `json:"PublicHost"`
	PublicPort string `json:"PublicPort"`
	PublicAddr string `json:"PublicAddr"`
}

// publicHostPort 从报文中提取公网 host:port，兼容新旧两种字段；无法解析则返回空。
func (p stunPayload) publicHostPort() (string, int) {
	host, port := p.PublicHost, p.PublicPort
	if host == "" || port == "" {
		// 回退到旧格式的 PublicAddr
		addr := p.PublicAddr
		if i := lastColon(addr); i > 0 {
			host = addr[:i]
			port = addr[i+1:]
		}
	}
	if host == "" || port == "" {
		return "", 0
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return "", 0
	}
	return host, n
}

// lastColon 返回字符串中最后一个 ':' 的位置，用于从 host:port 拆分（兼容 IPv4 与 hostname）。
func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// RunSync 供定时任务模块通过 HTTP API（/api/systemJobs/stunPortForwardSync）触发；
// 未启用（需 MQTT 已启用）时直接跳过。
func RunSync() {
	if !enabled() {
		slog.Info("stun 端口转发同步跳过", "reason", "未启用（需 MQTT 已启用）")
		return
	}
	run()
}

// interval 返回同步间隔：读取配置，非法值时回退到 30 秒。
func interval() time.Duration {
	sec := config.GetConfig().StunPortForwardSyncIntervalSec
	if sec <= 0 {
		sec = 30
	}
	return time.Duration(sec) * time.Second
}

// enabled 控制任务是否启动：仅当 MQTT 已启用时生效。
func enabled() bool {
	return config.GetConfig().Mqtt.Enabled
}

// run 执行一次同步：读取每个 STUN channel 的最新消息，
// 若端口转发中存在同名「直接转发」规则，则按消息里的公网地址更新目标地址；
// 规则处于启动状态时，先停止旧连接，更新数据库后再重新启动。
func run() {
	db := config.GetDB()
	if db == nil {
		slog.Warn("stun 端口转发同步跳过", "reason", "数据库不可用")
		return
	}

	msgs, err := mqttsync.ListLatestPerChannelLike(stunTopicPrefix)
	if err != nil {
		slog.Error("读取 STUN 消息失败", "err", err)
		return
	}
	slog.Info("stun 端口转发同步开始", "channels", len(msgs))

	for _, msg := range msgs {
		var payload stunPayload
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			slog.Warn("解析 STUN 消息失败，跳过", "channel", msg.Channel, "err", err)
			continue
		}
		publicHost, publicPort := payload.publicHostPort()
		if publicHost == "" {
			slog.Warn("STUN 消息缺少公网地址，跳过", "channel", msg.Channel,
				"hasPublicHost", payload.PublicHost != "", "hasPublicAddr", payload.PublicAddr != "")
			continue
		}

		var rule portfwd.PortForwarding
		if result := db.First(&rule, "name = ?", msg.Channel); result.Error != nil {
			// 没有同名端口转发规则，无需处理
			slog.Debug("未找到同名端口转发规则，跳过", "channel", msg.Channel)
			continue
		}

		// 只处理「直接转发」规则（不经 SSH 隧道，目标地址就是公网地址）
		if rule.Direction != portfwd.DirectionDirect {
			slog.Debug("STUN 消息对应规则非直接转发，跳过", "channel", msg.Channel, "direction", rule.Direction)
			continue
		}
		// 监听地址兜底：直接转发缺省监听全部网卡
		if strings.TrimSpace(rule.BindAddress) == "" {
			rule.BindAddress = portfwd.DefaultDirectBindAddress
		}

		// 目标地址没有变化，无需重启
		if rule.TargetHost == publicHost && rule.TargetPort == publicPort {
			continue
		}

		// 记录「配置是否启用」与「当前是否运行」；运行中的先停掉旧连接。
		configuredEnabled := rule.Status
		running, _ := portfwd.GlobalPortForwarder.Status(rule.ID)
		if running {
			if err := portfwd.GlobalPortForwarder.RemoveForward(rule.ID); err != nil {
				slog.Warn("停止旧端口转发失败", "ruleId", rule.ID, "channel", msg.Channel, "err", err)
				continue
			}
		}

		// 更新数据库中的目标地址（保留原有配置启用状态，不翻转）
		rule.TargetHost = publicHost
		rule.TargetPort = publicPort
		if result := db.Save(&rule); result.Error != nil {
			slog.Error("更新端口转发目标地址失败", "ruleId", rule.ID, "channel", msg.Channel, "err", result.Error)
			continue
		}
		slog.Info("端口转发目标地址已更新",
			"ruleId", rule.ID, "channel", msg.Channel,
			"target", publicHost+":"+strconv.Itoa(publicPort), "wasRunning", running)

		// 规则配置为启动状态时，重新建立连接
		if configuredEnabled {
			if err := portfwd.GlobalPortForwarder.AddForward(&rule, nil); err != nil {
				slog.Error("重新启动端口转发失败", "ruleId", rule.ID, "channel", msg.Channel, "err", err)
				continue
			}
			slog.Info("端口转发已重新启动", "ruleId", rule.ID, "channel", msg.Channel)
		}
	}
}
