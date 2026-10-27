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

var (
	client      mqtt.Client
	connectedMu sync.RWMutex
	connected   bool

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
	connectedMu.RLock()
	defer connectedMu.RUnlock()
	return connected
}

func setConnected(v bool) {
	connectedMu.Lock()
	connected = v
	connectedMu.Unlock()
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
	localNodeID = id
	return nil
}

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
	slog.Info("MQTT 正在连接 broker（失败会在后台重试）", "broker", cfg.Broker)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		setConnected(false)
		slog.Warn("MQTT 连接失败（应用继续运行，本地功能不受影响）", "broker", cfg.Broker, "err", token.Error())
		return
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
