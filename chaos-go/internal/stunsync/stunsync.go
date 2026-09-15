package stunsync

import (
	"fmt"
	"log/slog"
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
	// 完整 topic：{MQTT_PREFIX}stun/{hostname}
	stunTopicChannel = "stun"
)

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

	// 封装为 wireMessage 并广播到 {prefix}stun/{hostname}。
	// 本机订阅会因 node_id 一致被静默丢弃，不会触发自回环告警；对端节点正常接收。
	channel := stunTopicChannel + "/" + host
	topic := mqttsync.Prefix() + channel
	if err := mqttsync.PublishLocal(mqttsync.NewWireMessage(channel, string(body))); err != nil {
		// broker 离线：本地已落库，仅记日志，下次 tick 重试
		slog.Warn("stun 规则列表发布 MQTT 失败", "topic", topic, "err", err)
		return
	}
	slog.Info("stun 规则列表已发布到 MQTT", "topic", topic, "bytes", len(body))
}
