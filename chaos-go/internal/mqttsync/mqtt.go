package mqttsync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"chaos-go/config"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
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

	nodeMu      sync.RWMutex
	localNodeID string

	// 加密状态：Start 后一次性确定，之后只读（高频路径不加锁读取）
	encryptMu        sync.RWMutex
	encryptKey       []byte
	effectiveEncrypt bool
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

const aesGCMNonceSize = 12

// envelope 是加密后的外层信封；无 enc 字段的报文视为明文旧消息（兼容滚动升级）。
type envelope struct {
	V    int    `json:"v"`
	Enc  string `json:"enc"`
	Data string `json:"data"` // base64(nonce || ciphertext+tag)
}

// loadKey 将 .env 中的 MQTT_ENCRYPT_KEY 解析为 32 字节密钥。
// 支持 hex（64 字符）或 base64（解出 32 字节）两种编码；无效返回 nil。
func loadKey(raw string) []byte {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	return nil
}

// initCrypto 依据配置确定本会话是否真正启用加密。
// Encrypt=true 且密钥有效 → 启用；否则明文降级（密钥无效时记错误日志，仍照常运行）。
func initCrypto(cfg config.MqttConfig) {
	encryptMu.Lock()
	defer encryptMu.Unlock()
	if !cfg.Encrypt {
		effectiveEncrypt = false
		encryptKey = nil
		return
	}
	key := loadKey(cfg.EncryptKey)
	if len(key) != 32 {
		slog.Error("MQTT_ENCRYPT=true 但 MQTT_ENCRYPT_KEY 无效（需 32 字节，hex 或 base64），本会话按明文运行",
			"keyLen", len(key))
		effectiveEncrypt = false
		encryptKey = nil
		return
	}
	effectiveEncrypt = true
	encryptKey = key
}

// EffectiveEncrypt 返回本会话是否真正启用加密（发布与状态读取用，线程安全）。
func EffectiveEncrypt() bool {
	encryptMu.RLock()
	defer encryptMu.RUnlock()
	return effectiveEncrypt
}

// seal 用 AES-256-GCM 加密明文内层报文，返回外层信封 JSON 字节。
func seal(plain []byte) ([]byte, error) {
	encryptMu.RLock()
	key := encryptKey
	encryptMu.RUnlock()
	if len(key) != 32 {
		return nil, errors.New("mqttsync: encryption key not initialized")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plain, nil)
	env := envelope{V: 1, Enc: "aes-256-gcm", Data: base64.StdEncoding.EncodeToString(append(nonce, ct...))}
	return json.Marshal(env)
}

// open 解密外层信封，返回内层明文报文。
// 若报文不含 enc 字段（明文旧消息），则原样返回，兼容集群滚动升级。
// 解密失败（篡改 / 无密钥注入）返回错误，调用方应丢弃该消息。
func open(wrapped []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(wrapped, &env); err != nil {
		// 非 JSON（极少见）：按明文兼容处理
		return wrapped, nil
	}
	if env.Enc != "aes-256-gcm" {
		// 明文旧消息：直接返回原始字节，交由业务层解析 wireMessage
		return wrapped, nil
	}
	encryptMu.RLock()
	key := encryptKey
	encryptMu.RUnlock()
	if len(key) != 32 {
		return nil, errors.New("mqttsync: encryption key not initialized")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("mqttsync: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err // 篡改 / 无密钥 → 失败
	}
	return plain, nil
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
	initCrypto(cfg)

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

	inner, err := open(msg.Payload())
	if err != nil {
		slog.Warn("MQTT 报文解密失败，已丢弃（疑似篡改或非法注入）", "topic", msg.Topic(), "err", err)
		return
	}
	var m wireMessage
	if err := json.Unmarshal(inner, &m); err != nil {
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
	inner, err := json.Marshal(m)
	if err != nil {
		return err
	}
	payload := inner
	if EffectiveEncrypt() {
		if enc, e := seal(inner); e != nil {
			return e
		} else {
			payload = enc
		}
	}
	token := client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}
