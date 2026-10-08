package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppConfig 是 chaos-go 的全部启动配置。
// 配置默认从运行目录下的 configs/config.yaml 加载，可用环境变量
// CONFIG_FILE 指定其它路径；文件缺失或解析失败时回退到内置默认值。
type AppConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"db"`
	Pprof    PprofConfig    `yaml:"pprof"`
	Mqtt     MqttConfig     `yaml:"mqtt"`
	Supabase SupabaseConfig `yaml:"supabase"`
	Log      LogConfig      `yaml:"log"`
	Feature  FeatureConfig  `yaml:"feature"`
	DeepSeek DeepSeekConfig `yaml:"deepseek"`
	Baidu    BaiduConfig    `yaml:"baidu"`
	Lucky    LuckyConfig    `yaml:"lucky"`
	Stun     StunConfig     `yaml:"stun"`
	Note     NoteConfig     `yaml:"note"`
}

type ServerConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	EnableHTTP3 bool   `yaml:"enable_http3"` // 同时启用 HTTPS(TCP) + HTTP/3(UDP/QUIC)，强制 TLS
	HTTP3Port   int    `yaml:"http3_port"`   // HTTP/3 的 UDP 端口，0 表示与 Port 相同
	TLSCertFile string `yaml:"tls_cert_file"` // 受信任证书路径（可选），留空则自动生成自签名证书
	TLSKeyFile  string `yaml:"tls_key_file"`  // 私钥路径（可选）
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`     // 数据库类型：postgres / sqlite
	Host     string `yaml:"host"`     // postgres：主机地址
	Port     int    `yaml:"port"`     // postgres：端口
	User     string `yaml:"user"`     // postgres：用户名
	Password string `yaml:"password"` // postgres：密码
	DBName   string `yaml:"name"`     // postgres：数据库名
	SSLMode  string `yaml:"sslmode"`  // postgres：SSL 模式
	Path     string `yaml:"path"`     // sqlite：数据库文件路径（相对运行目录，目录自动创建）
}

type PprofConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

// SupabaseConfig 云端数据通道（Supabase Data API / PostgREST）配置。
// SecretKey 为后端专用凭据，只进 configs/config.yaml（已被 .gitignore 忽略），禁止写入任何被版本控制的文件。
type SupabaseConfig struct {
	URL            string     `yaml:"url"`             // 项目地址，形如 https://<project-ref>.supabase.co
	SecretKey      string     `yaml:"secret_key"`      // sb_secret_*：绕过 RLS，仅后端使用
	PublishableKey string     `yaml:"publishable_key"` // sb_publishable_*：受 RLS 约束，保留给将来的前端只读场景
	Schema         string     `yaml:"schema"`          // 目标 schema，缺省 public
	TimeoutSec     int        `yaml:"timeout_sec"`     // 单次请求超时秒数，缺省 15
	Tables         StringList `yaml:"tables"`          // 允许访问的表名清单（逗号分隔字符串或 YAML 列表），空表示该通道不可用
}

// StringList 兼容「逗号分隔字符串」与「YAML 列表」两种写法，
// 解析时去除每项两端空白并丢弃空项。
type StringList []string

func (s *StringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		parts := strings.Split(value.Value, ",")
		items := make(StringList, 0, len(parts))
		for _, part := range parts {
			if item := strings.TrimSpace(part); item != "" {
				items = append(items, item)
			}
		}
		*s = items
		return nil
	case yaml.SequenceNode:
		var raw []string
		if err := value.Decode(&raw); err != nil {
			return err
		}
		items := make(StringList, 0, len(raw))
		for _, part := range raw {
			if item := strings.TrimSpace(part); item != "" {
				items = append(items, item)
			}
		}
		*s = items
		return nil
	default:
		*s = StringList{}
		return nil
	}
}

// Available 判定云端数据通道是否可用：项目地址、后端凭据与表名清单缺一不可。
func (c *SupabaseConfig) Available() bool {
	return c.URL != "" && c.SecretKey != "" && len(c.Tables) > 0
}

// MqttConfig 多节点消息同步通道（基于公共 MQTT broker）配置。
// Broker / Prefix 为公开信息；Username/Password 仅进 configs/config.yaml，禁止写入版本控制文件。
type MqttConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 功能总开关，缺省 false
	Broker     string `yaml:"broker"`      // broker 地址
	Prefix     string `yaml:"prefix"`      // 集群公共主题前缀，缺省 test/
	ClientID   string `yaml:"client_id"`   // 缺省 chaos-<nodeID>
	Username   string `yaml:"username"`    // 公共 broker 多为匿名，留空
	Password   string `yaml:"password"`    // 同上
	Encrypt    bool   `yaml:"encrypt"`     // 是否启用 AES-256-GCM 载荷加密
	EncryptKey string `yaml:"encrypt_key"` // 32 字节共享密钥（hex 或 base64），仅进 configs/config.yaml，禁止入库/日志
}

// LogConfig 日志输出配置。
type LogConfig struct {
	Level     string `yaml:"level"`      // 日志级别（debug/info/warn/error），缺省 info
	FilePath  string `yaml:"file_path"`  // 日志文件路径，缺省 logs/app.log
	ToFile    bool   `yaml:"to_file"`    // 是否写文件
	ToConsole bool   `yaml:"to_console"` // 是否写控制台
}

// FeatureConfig 功能开关。
type FeatureConfig struct {
	FileLink  bool `yaml:"file_link"`  // 文件连接功能，缺省 true
	CronShell bool `yaml:"cron_shell"` // 定时任务「执行命令/脚本」动作总开关，缺省 false（危险能力，按需开启）
}

// DeepSeekConfig DeepSeek API 配置（用于余额查询等）。
type DeepSeekConfig struct {
	APIKey string `yaml:"api_key"`
}

// BaiduConfig 百度地图 API 配置（用于手机小看板天气）。
type BaiduConfig struct {
	AK string `yaml:"ak"`
}

// LuckyConfig Lucky STUN 规则列表定时同步到 MQTT 的相关配置。
// 仅当 MQTT 启用且已配置 OpenToken 时，后台任务才会启动。
type LuckyConfig struct {
	OpenToken       string `yaml:"open_token"`        // Lucky 服务的 openToken，用于请求 stunrulelist 接口
	SyncIntervalSec int    `yaml:"sync_interval_sec"` // 同步间隔（秒），缺省 60
}

// NoteConfig 笔记库（Vault）配置。
// RootPath 是本机敏感路径，只允许出现在未被版本控制的 configs/config.yaml。
type NoteConfig struct {
	Enabled     bool       `yaml:"enabled"`      // 模块开关，缺省 false
	RootPath    string     `yaml:"root_path"`    // Vault 根目录，缺省空（模块不可用）
	IncludeExt  StringList `yaml:"include_ext"`  // 纳入索引的扩展名清单，缺省 md,markdown,txt
	IgnoreGlobs StringList `yaml:"ignore_globs"` // 忽略的目录 / 文件名清单，缺省见 DefaultNoteIgnores
	MaxFileSize int        `yaml:"max_file_size"`
}

// 笔记模块的缺省值：配置缺省或异常时回退到这里。
const (
	DefaultNoteMaxFileSize = 2 * 1024 * 1024 // 2MB
)

// DefaultNoteIgnores 默认忽略的目录 / 文件（避免把版本控制与工具元数据扫进索引）。
var DefaultNoteIgnores = []string{".git", "node_modules", ".obsidian", ".trash", ".idea", ".vscode"}

// DefaultNoteExts 默认纳入索引的扩展名（不带点）。
var DefaultNoteExts = []string{"md", "markdown", "txt"}

// Available 判定笔记模块是否可用：开关打开且已配置 Vault 根目录。
func (c *NoteConfig) Available() bool {
	return c.Enabled && strings.TrimSpace(c.RootPath) != ""
}

// Root 返回 Vault 根目录的绝对路径；模块不可用时返回空串。
func (c *NoteConfig) Root() string {
	if !c.Available() {
		return ""
	}
	abs, err := filepath.Abs(c.RootPath)
	if err != nil {
		slog.Warn("笔记库根目录解析失败，模块不可用", "rootPath", c.RootPath, "err", err)
		return ""
	}
	return abs
}

// Exts 返回纳入索引的扩展名（小写、无前导点）；未配置时回退默认值。
func (c *NoteConfig) Exts() []string {
	if len(c.IncludeExt) == 0 {
		return DefaultNoteExts
	}
	exts := make([]string, 0, len(c.IncludeExt))
	for _, e := range c.IncludeExt {
		if e = strings.TrimSpace(strings.TrimPrefix(e, ".")); e != "" {
			exts = append(exts, strings.ToLower(e))
		}
	}
	if len(exts) == 0 {
		return DefaultNoteExts
	}
	return exts
}

// Ignores 返回忽略的目录 / 文件名清单；未配置时回退默认值。
func (c *NoteConfig) Ignores() []string {
	if len(c.IgnoreGlobs) == 0 {
		return DefaultNoteIgnores
	}
	return []string(c.IgnoreGlobs)
}

// LimitBytes 返回单文件大小上限（字节）；非正数回退默认值。
func (c *NoteConfig) LimitBytes() int {
	if c.MaxFileSize <= 0 {
		return DefaultNoteMaxFileSize
	}
	return c.MaxFileSize
}

// StunConfig STUN 公网地址同步到端口转发目标的定时任务配置。
// 仅当 MQTT 启用时，后台任务才会启动。
type StunConfig struct {
	PortForwardSyncIntervalSec int `yaml:"port_forward_sync_interval_sec"` // 同步间隔（秒），缺省 30
}

var globalConfig *AppConfig

// defaultConfig 返回内置默认值，文件中的字段会在其之上覆盖。
func defaultConfig() *AppConfig {
	return &AppConfig{
		Server:   ServerConfig{Host: "0.0.0.0", Port: 8080},
		Database: DatabaseConfig{Type: "postgres", Host: "localhost", Port: 5432, User: "postgres", DBName: "chaos", SSLMode: "disable", Path: "chaos.db"},
		Pprof:    PprofConfig{Host: "localhost", Port: 6060},
		Supabase: SupabaseConfig{Schema: "public", TimeoutSec: 15},
		Log:      LogConfig{Level: "info", FilePath: "logs/app.log"},
		Feature:  FeatureConfig{FileLink: true},
		Lucky:    LuckyConfig{SyncIntervalSec: 60},
		Stun:     StunConfig{PortForwardSyncIntervalSec: 30},
		Note:     NoteConfig{Enabled: false, MaxFileSize: DefaultNoteMaxFileSize},
	}
}

func LoadConfig() *AppConfig {
	if globalConfig != nil {
		return globalConfig
	}

	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = filepath.Join(getConfigDir(), "configs", "config.yaml")
	}

	config := defaultConfig()

	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			slog.Warn("解析配置文件失败，使用默认配置", "path", configPath, "err", err)
		}
	} else {
		slog.Warn("配置文件不存在，使用默认配置", "path", configPath)
	}

	// 日志通道：两者都未显式开启时，默认全部开启（兼容旧逻辑）
	if !config.Log.ToFile && !config.Log.ToConsole {
		config.Log.ToFile = true
		config.Log.ToConsole = true
	}

	globalConfig = config
	slog.Info("配置加载成功", "configPath", configPath)
	return config
}

func GetConfig() *AppConfig {
	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}

func getConfigDir() string {
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}

	execPath, err := os.Executable()
	if err == nil {
		return filepath.Dir(execPath)
	}

	return "."
}

func (c *DatabaseConfig) GetDSN() string {
	if c.Type == "sqlite" {
		return c.Path
	}
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		c.Host, c.User, c.Password, c.DBName, c.Port, c.SSLMode,
	)
}

func (c *ServerConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetHTTP3Port 返回 HTTP/3 的 UDP 监听端口；为 0 时与 Port 一致。
func (c *ServerConfig) GetHTTP3Port() int {
	if c.HTTP3Port > 0 {
		return c.HTTP3Port
	}
	return c.Port
}

// GetHTTP3Address 返回 HTTP/3(UDP/QUIC) 监听地址。
func (c *ServerConfig) GetHTTP3Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.GetHTTP3Port())
}

func (c *PprofConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func InitLog() {
	cfg := GetConfig()
	if cfg == nil {
		return
	}

	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	writers := []io.Writer{os.Stderr}

	if cfg.Log.ToFile && cfg.Log.FilePath != "" {
		dir := filepath.Dir(cfg.Log.FilePath)
		os.MkdirAll(dir, 0o755)
		f, err := os.OpenFile(cfg.Log.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Warn("failed to open log file, falling back to stderr only", "path", cfg.Log.FilePath, "error", err)
		} else {
			slog.Info("log file configured", "path", cfg.Log.FilePath)
			writers = append(writers, f)
		}
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
