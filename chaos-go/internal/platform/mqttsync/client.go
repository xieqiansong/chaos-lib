package mqttsync

import (
	"chaos-go/pkg/tools"
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

	"chaos-go/internal/framework/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	watchdogInterval = 10 * time.Second
	maxReconnectWait = 30 * time.Second
	opWaitTimeout    = 10 * time.Second
	connectStuckTTL  = 60 * time.Second
	stuckOfflineTTL  = 2 * time.Minute
)

type connManager struct {
	mu         sync.RWMutex
	client     mqtt.Client
	generation uint64

	tokenMu      sync.Mutex
	connectToken mqtt.Token
	connectAt    time.Time

	subMu       sync.Mutex
	subscribed  bool
	subscribing bool

	// offlineSince 仅由 watchdog 协程读写
	offlineSince time.Time
}

var (
	connMgr      = &connManager{}
	watchdogOnce sync.Once

	nodeMu      sync.RWMutex
	localNodeID string

	encryptMu        sync.RWMutex
	encryptKey       []byte
	effectiveEncrypt bool
)

func NodeID() string {
	nodeMu.RLock()
	defer nodeMu.RUnlock()
	return localNodeID
}

func IsConnected() bool {
	c := connMgr.current()
	return c != nil && c.IsConnectionOpen()
}

func (m *connManager) current() mqtt.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

func (m *connManager) isCurrent(gen uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generation == gen
}

type envelope struct {
	V    int    `json:"v"`
	Enc  string `json:"enc"`
	Data string `json:"data"`
}

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

func EffectiveEncrypt() bool {
	encryptMu.RLock()
	defer encryptMu.RUnlock()
	return effectiveEncrypt
}

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

func open(wrapped []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(wrapped, &env); err != nil {

		return wrapped, nil
	}
	if env.Enc != "aes-256-gcm" {

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
		return nil, err
	}
	return plain, nil
}

func normalizePrefix(p string) string {
	if !strings.HasSuffix(p, "/") {
		return p + "/"
	}
	return p
}

func loadNodeID() error {
	id, err := GetOrCreateNodeID()
	if err != nil {
		return err
	}
	nodeMu.Lock()
	localNodeID = id
	nodeMu.Unlock()
	return nil
}

func Start() {
	cfg := config.GetConfig().Mqtt
	if !cfg.Enabled {
		slog.Info("MQTT 同步未启用")
		return
	}
	initCrypto(cfg)
	if err := loadNodeID(); err != nil {
		slog.Warn("MQTT 加载节点标识失败，后台会重试", "err", err)
	}
	slog.Info("MQTT 正在连接 broker（失败会在后台重试）")
	watchdogOnce.Do(func() { go connMgr.run() })
}

func buildOptions(cfg config.MqttConfig, gen uint64) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(clientID(cfg))
	opts.SetAutoReconnect(true)
	// paho 默认最大重连间隔 10 分钟且逐次指数退避，网络恢复后仍可能长时间不重试
	opts.SetMaxReconnectInterval(maxReconnectWait)
	// 首次连接重试交给 watchdog：ConnectRetry 会让 Connect token 一直不完成，
	// 且 connecting 状态下发生连接丢失时 paho 不会启动自动重连
	opts.SetConnectRetry(false)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetDefaultPublishHandler(onMessage)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		if !connMgr.isCurrent(gen) {
			return
		}
		connMgr.resetSubscription()
		slog.Info("MQTT 已连接")
		connMgr.ensureSubscribed(c)
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		if !connMgr.isCurrent(gen) {
			return
		}
		connMgr.resetSubscription()
		slog.Warn("MQTT 连接丢失", "err", err)
	})
	return opts
}

func (m *connManager) install() (mqtt.Client, mqtt.Client, uint64) {
	cfg := config.GetConfig().Mqtt
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generation++
	gen := m.generation
	old := m.client
	c := mqtt.NewClient(buildOptions(cfg, gen))
	m.client = c
	return old, c, gen
}

func (m *connManager) tryConnect(c mqtt.Client) {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()
	if m.connectToken != nil {
		select {
		case <-m.connectToken.Done():
		default:
			return
		}
	}
	m.connectToken = c.Connect()
	m.connectAt = time.Now()
}

func (m *connManager) clearConnectToken() {
	m.tokenMu.Lock()
	m.connectToken = nil
	m.tokenMu.Unlock()
}

func (m *connManager) resetSubscription() {
	m.subMu.Lock()
	m.subscribed = false
	m.subMu.Unlock()
}

func (m *connManager) ensureSubscribed(c mqtt.Client) {
	m.subMu.Lock()
	if m.subscribed || m.subscribing {
		m.subMu.Unlock()
		return
	}
	m.subscribing = true
	m.subMu.Unlock()

	go func() {
		token := c.Subscribe(Prefix()+"#", 1, nil)
		ok := token.WaitTimeout(opWaitTimeout)
		err := token.Error()
		if m.current() != c {
			m.subMu.Lock()
			m.subscribing = false
			m.subMu.Unlock()
			return
		}

		m.subMu.Lock()
		m.subscribing = false
		if ok && err == nil {
			m.subscribed = true
		}
		m.subMu.Unlock()

		switch {
		case !ok:
			slog.Warn("MQTT 订阅超时，稍后重试")
		case err != nil:
			slog.Warn("MQTT 订阅失败，稍后重试", "err", err)
		default:
			slog.Info("MQTT 已订阅")
		}
	}()
}

func (m *connManager) rebuild(reason string) {
	old, c, gen := m.install()
	m.resetSubscription()
	m.clearConnectToken()
	m.offlineSince = time.Now()
	if old == nil {
		slog.Info("MQTT 客户端已初始化", "generation", gen)
	} else {
		// 停止旧客户端的后台重连，避免相同 ClientID 互相挤下线；
		// Disconnect 可能因旧协程卡住而阻塞，放到后台执行
		go old.Disconnect(0)
		slog.Warn("MQTT 客户端已重建", "reason", reason, "generation", gen)
	}
	m.tryConnect(c)
}

func (m *connManager) run() {
	m.tick()
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.tick()
	}
}

func (m *connManager) tick() {
	if NodeID() == "" {
		if err := loadNodeID(); err != nil {
			slog.Warn("MQTT 节点标识未就绪，等待重试", "err", err)
			return
		}
		m.rebuild("节点标识就绪")
		return
	}

	c := m.current()
	if c == nil {
		m.rebuild("客户端未初始化")
		return
	}

	if c.IsConnectionOpen() {
		if !m.offlineSince.IsZero() {
			slog.Info("MQTT 连接已恢复", "offline", time.Since(m.offlineSince).Round(time.Second).String())
			m.offlineSince = time.Time{}
		}
		m.clearConnectToken()
		m.ensureSubscribed(c)
		return
	}

	if m.offlineSince.IsZero() {
		m.offlineSince = time.Now()
	}

	m.tokenMu.Lock()
	tok, tokAt := m.connectToken, m.connectAt
	m.tokenMu.Unlock()
	if tok != nil {
		select {
		case <-tok.Done():
			if err := tok.Error(); err != nil {
				slog.Debug("MQTT 连接尝试失败，稍后重试", "err", err)
			}
			m.clearConnectToken()
		default:
			if time.Since(tokAt) < connectStuckTTL {
				return
			}
			slog.Warn("MQTT 连接尝试长时间无响应，重建客户端")
			m.rebuild("连接尝试超时")
			return
		}
	}

	if !c.IsConnected() {
		// paho 已放弃自动重连（disconnected），主动再发起一次连接
		m.tryConnect(c)
		return
	}
	// paho 仍处于 connecting/reconnecting，但迟迟没有恢复
	if time.Since(m.offlineSince) > stuckOfflineTTL {
		slog.Warn("MQTT 自动重连长时间未恢复，重建客户端")
		m.rebuild("自动重连超时")
	}
}

func clientID(cfg config.MqttConfig) string {
	if cfg.ClientID != "" {
		return cfg.ClientID
	}
	return "chaos-" + NodeID()
}

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

	if m.NodeID != "" && m.NodeID == NodeID() {
		return
	}
	if _, err := saveMessage(m); err != nil {
		slog.Error("MQTT 收到消息落库失败", "err", err)
		return
	}
	slog.Info("MQTT 收到对端消息", "channel", channel, "node", m.NodeID)
}

func Publish(m wireMessage) error {
	c := connMgr.current()
	if c == nil || !c.IsConnectionOpen() {
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
	token := c.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(opWaitTimeout) {
		return errors.New("mqttsync: publish timeout")
	}
	return token.Error()
}

func Prefix() string {
	return normalizePrefix(config.GetConfig().Mqtt.Prefix)
}

func PublishLocal(m wireMessage) error {
	if _, err := saveMessage(m); err != nil {
		slog.Error("MQTT 消息本地落库失败", "channel", m.Channel, "err", err)
	}
	if err := Publish(m); err != nil {

		slog.Warn("MQTT 发布失败（本地已落库）", "channel", m.Channel, "err", err)
		return err
	}
	return nil
}

func NewWireMessage(channel, payload string) wireMessage {
	return wireMessage{
		ID:      tools.UUID(),
		NodeID:  NodeID(),
		Channel: channel,
		Payload: payload,
		Ts:      time.Now().UTC().Format(time.RFC3339),
	}
}
