package stunsync

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"chaos-go/config"
	"chaos-go/internal/mqttsync"
	"chaos-go/scheduler"
	"chaos-go/tools"
)

const (
	stunAPIBase = "http://localhost:16601/api/stunrulelist"
	// stunTopicChannel 是 topic 片段，配合 MQTT 公共前缀与主机名 =>
	// 每条规则独立发布到 {MQTT_PREFIX}stun/{hostname}/{Name}，便于对端按名称精确订阅。
	stunTopicChannel = "stun"
)

// stunRuleListResp 对应 /api/stunrulelist 的返回结构。
type stunRuleListResp struct {
	ModuleEnable bool              `json:"ModuleEnable"`
	List         []json.RawMessage `json:"list"`
}

// stunRule 解析 list 元素的必要字段，用于过滤与精简发布内容。
type stunRule struct {
	Enable         bool     `json:"Enable"`
	Name           string   `json:"Name"`
	TargetAddrList []string `json:"TargetAddrList"`
	PublicAddr     string   `json:"PublicAddr"`
}

// stunRuleTrim 是发布到 MQTT 的精简报文，仅保留 Name / TargetHost / TargetPort / PublicHost / PublicPort。
type stunRuleTrim struct {
	Name       string `json:"Name"`
	TargetHost string `json:"TargetHost"`
	TargetPort string `json:"TargetPort"`
	PublicHost string `json:"PublicHost"`
	PublicPort string `json:"PublicPort"`
}

// splitHostPort 将 "host:port" 拆成 host / port；解析失败时 host 取原串、port 为空。
func splitHostPort(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return host, port
}

func init() {
	scheduler.Register("stun-rule-sync", interval(), run, enabled)
}

// interval 返回同步间隔：读取配置，非法值时回退到 60 秒。
func interval() time.Duration {
	sec := config.GetConfig().LuckySyncIntervalSec
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// enabled 控制任务是否启动：仅当 MQTT 已启用且已配置 LUCKY_OPEN_TOKEN 时生效。
func enabled() bool {
	cfg := config.GetConfig()
	return cfg.Mqtt.Enabled && cfg.LuckyOpenToken != ""
}

// run 执行一次同步：拉取 stun 规则列表并发布到 MQTT。
func run() {
	cfg := config.GetConfig()

	url := fmt.Sprintf("%s?openToken=%s", stunAPIBase, cfg.LuckyOpenToken)
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

	// 全局开关 ModuleEnable 仍发布到基础 topic {prefix}stun/{hostname}，
	// 原整包里就含该字段，拆分后单独保留以免对端丢失全局启用状态。
	baseChannel := stunTopicChannel + "/" + host
	baseTopic := mqttsync.Prefix() + baseChannel

	// 逐条拆分 list，每条规则独立发布到 {prefix}stun/{hostname}/{Name}。
	// 仅发布 Enable = true 的规则，且内容精简为 Name / TargetAddrList / PublicAddr。
	// 本机订阅会因 node_id 一致被静默丢弃，不会触发自回环告警；对端节点按名称订阅。
	sent := 0
	for _, raw := range resp.List {
		var rule stunRule
		if e := json.Unmarshal(raw, &rule); e != nil || rule.Name == "" {
			slog.Warn("stun 规则缺少 Name，跳过", "err", e)
			continue
		}
		// 仅发送启用的规则（Enable = true），未启用的直接跳过。
		if !rule.Enable {
			continue
		}
		// 精简报文：只保留 Name / TargetHost / TargetPort / PublicHost / PublicPort。
		// TargetAddrList 仅取第一个元素，再按 host:port 拆分；PublicAddr 同理。
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
			// broker 离线：本地已落库，仅记日志，下条与下次 tick 重试；
			// 断连状态下后续规则必败，直接退出循环避免重复告警。
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
		slog.Info("stun 规则列表已拆分发布到 MQTT", "topic", baseTopic+"/*", "total", len(resp.List), "sent", sent)
	}
}
