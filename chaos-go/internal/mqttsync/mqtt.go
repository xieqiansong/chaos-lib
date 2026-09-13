package mqttsync

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"chaos-go/config"
)

const defaultChannel = "broadcast"

var (
	// ErrDBUnavailable 表示数据库单例不可用（极端情况下）。
	ErrDBUnavailable = errors.New("mqttsync: database unavailable")
	// ErrNotConnected 表示 broker 当前未连接，消息仅本地落库。
	ErrNotConnected = errors.New("mqttsync: broker not connected")
)

var (
	client      mqtt.Client
	connectedMu sync.RWMutex
	connected   bool

	nodeMu     sync.RWMutex
	localNodeID string
)

// newID 生成消息/节点唯一标识。
func newID() string { return uuid.NewString() }

// NodeID 返回本机节点标识（Start 后稳定）。
func NodeID() string {
	nodeMu.RLock()
	defer nodeMu.RUnlock()
	return localNodeID
}

// IsConnected 返回当前 broker 连接状态。
func IsConnected() bool {
	connectedMu.RLock()
	defer connectedMu.RUnlock()
	return connected
}

// setConnected 更新连接状态，线程安全。
func setConnected(v bool) {
	connectedMu.Lock()
	connected = v
	connectedMu.Unlock()
}

// normalizePrefix 确保前缀以 "/" 结尾，便于拼接 topic。
func normalizePrefix(p string) string {
	if !strings.HasSuffix(p, "/") {
		return p + "/"
	}
	return p
}

// loadNodeID 从 mqtt_sync_node 表加载节点标识；不存在则生成并持久化。
func loadNodeID() error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	var node MqttSyncNode
	err := db.Order("id ASC").First(&node).Error
	if err == nil && node.NodeID != "" {
		localNodeID = node.NodeID
		return nil
	}
	// 无记录（或查询异常）：新生成一个并写入；异常时仍用内存态，不阻断启动
	id := newID()
	rec := MqttSyncNode{NodeID: id}
	if err2 := db.Create(&rec).Error; err2 != nil {
		slog.Warn("MQTT 节点标识持久化失败，使用内存态", "err", err2)
	}
	localNodeID = id
	return nil
}

// Start 在 main 中调用：仅 Enabled 时连接 broker 并订阅整个集群前缀。
// 任何失败都只记录、不 panic、不阻塞启动。
func Start() {
	cfg := config.GetConfig().Mqtt
	if !cfg.Enabled {
		slog.Info("MQTT 同步未启用")
		return
	}
	if err := loadNodeID(); err != nil {
		slog.Warn("MQTT 加载节点标识失败，跳过启动", "err", err)
		return
	}

	prefix := normalizePrefix(cfg.Prefix)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(clientID(cfg))
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetDefaultPublishHandler(onMessage)
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		setConnected(true)
		// 订阅集群前缀下的全部主题
		if token := client.Subscribe(prefix+"#", 1, nil); token.Wait() && token.Error() != nil {
			slog.Warn("MQTT 订阅失败", "topic", prefix+"#", "err", token.Error())
			return
		}
		slog.Info("MQTT 已连接并订阅", "prefix", prefix, "node", NodeID())
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		setConnected(false)
		slog.Warn("MQTT 连接丢失", "err", err)
	})

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		setConnected(false)
		slog.Warn("MQTT 连接失败（应用继续运行，本地功能不受影响）", "broker", cfg.Broker, "err", token.Error())
		return
	}
}

// clientID 计算 MQTT 客户端 ID，缺省 chaos-<nodeID>。
func clientID(cfg config.MqttConfig) string {
	if cfg.ClientID != "" {
		return cfg.ClientID
	}
	return "chaos-" + NodeID()
}

// onMessage 是订阅回调：解析报文、丢弃本机回声、去重落库。
func onMessage(_ mqtt.Client, msg mqtt.Message) {
	prefix := normalizePrefix(config.GetConfig().Mqtt.Prefix)
	channel := strings.TrimPrefix(msg.Topic(), prefix)

	var m wireMessage
	if err := json.Unmarshal(msg.Payload(), &m); err != nil {
		slog.Warn("MQTT 报文解析失败，已丢弃", "topic", msg.Topic(), "err", err)
		return
	}
	if m.ID == "" {
		slog.Warn("MQTT 报文缺少 id，已丢弃", "topic", msg.Topic())
		return
	}
	// 丢弃本机回声：发布路径已即时落库
	if m.NodeID != "" && m.NodeID == NodeID() {
		return
	}
	if _, err := saveMessage(m); err != nil {
		slog.Error("MQTT 收到消息落库失败", "err", err)
		return
	}
	slog.Info("MQTT 收到对端消息", "channel", channel, "node", m.NodeID)
}

// Publish 把消息发布到 prefix+channel（尽力而为）。broker 未连接时返回 ErrNotConnected，
// 调用方据此仅记日志、保留本地落库结果。
func Publish(m wireMessage) error {
	if client == nil || !client.IsConnected() {
		return ErrNotConnected
	}
	prefix := normalizePrefix(config.GetConfig().Mqtt.Prefix)
	topic := prefix + m.Channel
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	token := client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}
