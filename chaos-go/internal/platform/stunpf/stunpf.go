package stunpf

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaos-go/internal/domain/portfwd"
	"chaos-go/internal/framework/config"
	"chaos-go/internal/platform/mqttsync"
)

const stunTopicPrefix = "stun/"

type stunPayload struct {
	Name       string `json:"Name"`
	TargetHost string `json:"TargetHost"`
	TargetPort string `json:"TargetPort"`
	PublicHost string `json:"PublicHost"`
	PublicPort string `json:"PublicPort"`
	PublicAddr string `json:"PublicAddr"`
}

func (p stunPayload) publicHostPort() (string, int) {
	host, port := p.PublicHost, p.PublicPort
	if host == "" || port == "" {

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

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

func RunSync() {
	if !enabled() {
		slog.Debug("stun 端口转发同步跳过", "reason", "未启用（需 MQTT 已启用）")
		return
	}
	run()
}

func interval() time.Duration {
	sec := config.GetConfig().Stun.PortForwardSyncIntervalSec
	if sec <= 0 {
		sec = 30
	}
	return time.Duration(sec) * time.Second
}

func enabled() bool {
	return config.GetConfig().Mqtt.Enabled
}

func run() {
	db := config.GetDB()
	if db == nil {
		slog.Warn("stun 端口转发同步跳过", "reason", "数据库不可用")
		return
	}

	msgs, err := mqttsync.ListLatest(stunTopicPrefix)
	if err != nil {
		slog.Error("读取 STUN 消息失败", "err", err)
		return
	}
	slog.Debug("stun 端口转发同步开始", "channels", len(msgs))

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

			slog.Debug("未找到同名端口转发规则，跳过", "channel", msg.Channel)
			continue
		}

		if rule.Direction != portfwd.DirectionDirect {
			slog.Debug("STUN 消息对应规则非直接转发，跳过", "channel", msg.Channel, "direction", rule.Direction)
			continue
		}

		if strings.TrimSpace(rule.BindAddress) == "" {
			rule.BindAddress = portfwd.DefaultDirectBindAddress
		}

		if rule.TargetHost == publicHost && rule.TargetPort == publicPort {
			continue
		}

		configuredEnabled := rule.Status
		running, _ := portfwd.GlobalPortForwarder.Status(rule.ID)
		if running {
			if err := portfwd.GlobalPortForwarder.RemoveForward(rule.ID); err != nil {
				slog.Warn("停止旧端口转发失败", "ruleId", rule.ID, "channel", msg.Channel, "err", err)
				continue
			}
		}

		rule.TargetHost = publicHost
		rule.TargetPort = publicPort
		if result := db.Save(&rule); result.Error != nil {
			slog.Error("更新端口转发目标地址失败", "ruleId", rule.ID, "channel", msg.Channel, "err", result.Error)
			continue
		}
		slog.Info("端口转发目标地址已更新",
			"ruleId", rule.ID, "channel", msg.Channel,
			"target", publicHost+":"+strconv.Itoa(publicPort), "wasRunning", running)

		if configuredEnabled {
			if err := portfwd.GlobalPortForwarder.AddForward(&rule, nil); err != nil {
				slog.Error("重新启动端口转发失败", "ruleId", rule.ID, "channel", msg.Channel, "err", err)
				continue
			}
			slog.Info("端口转发已重新启动", "ruleId", rule.ID, "channel", msg.Channel)
		}
	}
}
