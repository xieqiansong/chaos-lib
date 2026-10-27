package stunsync

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/platform/mqttsync"
	"chaos-go/pkg/tools"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
)

const (
	stunAPIBase = "http://localhost:16601/api/stunrulelist"

	stunTopicChannel = "stun"
)

type stunRuleListResp struct {
	ModuleEnable bool              `json:"ModuleEnable"`
	List         []json.RawMessage `json:"list"`
}

type stunRule struct {
	Enable         bool     `json:"Enable"`
	Name           string   `json:"Name"`
	TargetAddrList []string `json:"TargetAddrList"`
	PublicAddr     string   `json:"PublicAddr"`
}

type stunRuleTrim struct {
	Name       string `json:"Name"`
	TargetHost string `json:"TargetHost"`
	TargetPort string `json:"TargetPort"`
	PublicHost string `json:"PublicHost"`
	PublicPort string `json:"PublicPort"`
}

func splitHostPort(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return host, port
}

func RunSync() {
	if !enabled() {
		slog.Debug("stun 规则同步跳过", "reason", "未启用（需 MQTT 已启用且配置 LUCKY_OPEN_TOKEN）")
		return
	}
	run()
}

func enabled() bool {
	cfg := config.GetConfig()
	return cfg.Mqtt.Enabled && cfg.Lucky.OpenToken != ""
}

func run() {
	cfg := config.GetConfig()

	url := fmt.Sprintf("%s?openToken=%s", stunAPIBase, cfg.Lucky.OpenToken)
	status, body, err := tools.HTTPDo(http.MethodGet, url, nil, nil)
	if err != nil {
		slog.Error("获取 stun 规则列表失败", "err", err)
		return
	}
	if status != http.StatusOK {
		slog.Warn("获取 stun 规则列表返回非 200", "status", status)
		return
	}

	host, err := os.Hostname()
	if err != nil {
		slog.Error("获取主机名失败", "err", err)
		return
	}

	var resp stunRuleListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		slog.Error("解析 stun 规则列表失败", "err", err)
		return
	}

	baseChannel := stunTopicChannel + "/" + host
	baseTopic := mqttsync.Prefix() + baseChannel

	sent := 0
	for _, raw := range resp.List {
		var rule stunRule
		if e := json.Unmarshal(raw, &rule); e != nil || rule.Name == "" {
			slog.Warn("stun 规则缺少 Name，跳过", "err", e)
			continue
		}

		if !rule.Enable {
			continue
		}

		var targetHost, targetPort, publicHost, publicPort string
		if len(rule.TargetAddrList) > 0 {
			targetHost, targetPort = splitHostPort(rule.TargetAddrList[0])
		}
		publicHost, publicPort = splitHostPort(rule.PublicAddr)
		trimPayload, e := json.Marshal(stunRuleTrim{
			Name:       rule.Name,
			TargetHost: targetHost,
			TargetPort: targetPort,
			PublicHost: publicHost,
			PublicPort: publicPort,
		})
		if e != nil {
			slog.Warn("stun 规则精简序列化失败，跳过", "name", rule.Name, "err", e)
			continue
		}
		channel := fmt.Sprintf("%s/%s/%s", stunTopicChannel, host, rule.Name)
		topic := mqttsync.Prefix() + channel
		if e := mqttsync.PublishLocal(mqttsync.NewWireMessage(channel, string(trimPayload))); e != nil {

			if e == mqttsync.ErrNotConnected {
				slog.Warn("stun 规则发布 MQTT 失败（broker 离线）", "topic", topic, "err", e)
				return
			}
			slog.Warn("stun 规则发布 MQTT 失败", "topic", topic, "err", e)
			continue
		}
		sent++
	}
	if sent > 0 {
		slog.Debug("stun 规则列表已拆分发布到 MQTT", "topic", baseTopic+"/*", "total", len(resp.List), "sent", sent)
	}
}
