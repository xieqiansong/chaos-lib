package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	envconfig "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type AppConfig struct {
	// 以下为独立子配置（挂了方法或被当参数传递，保留为独立类型）
	Server   ServerConfig
	Database DatabaseConfig
	Pprof    PprofConfig
	Mqtt     MqttConfig
	Supabase SupabaseConfig

	// 以下为原纯分组子 struct 内联后的字段，省去无意义的类型跳转
	LogLevel     string `env:"LOG_LEVEL" envDefault:"info"`
	LogFilePath  string `env:"LOG_FILE_PATH" envDefault:"logs/app.log"`
	LogToFile    bool   `env:"LOG_TO_FILE"`
	LogToConsole bool   `env:"LOG_TO_CONSOLE"`

	FeatureFileLink bool `env:"FEATURE_FILE_LINK" envDefault:"true"`

	// 定时任务模块的「执行命令/脚本」动作总开关，缺省关闭（危险能力，按需开启）。
	FeatureCronShell bool `env:"FEATURE_CRON_SHELL" envDefault:"false"`

	DeepSeekAPIKey string `env:"DEEPSEEK_API_KEY"`
	BaiduAK        string `env:"BAIDU_AK"`

	// Lucky STUN 规则列表定时同步到 MQTT 的相关配置。
	// 仅当 MQTT 启用且已配置 LUCKY_OPEN_TOKEN 时，后台任务才会启动。
	LuckyOpenToken       string `env:"LUCKY_OPEN_TOKEN"`                 // Lucky 服务的 openToken，用于请求 stunrulelist 接口
	LuckySyncIntervalSec int    `env:"LUCKY_SYNC_INTERVAL_SEC" envDefault:"60"` // 同步间隔（秒），缺省 60

	// STUN 公网地址同步到端口转发目标的定时任务配置。
	// 仅当 MQTT 启用时，后台任务才会启动。
	StunPortForwardSyncIntervalSec int `env:"STUN_PORT_FORWARD_SYNC_INTERVAL_SEC" envDefault:"30"` // 同步间隔（秒），缺省 30
}

type ServerConfig struct {
	Port int    `env:"SERVER_PORT" envDefault:"8080"`
	Host string `env:"SERVER_HOST" envDefault:"0.0.0.0"`
}

type DatabaseConfig struct {
	Type     string `env:"DB_TYPE" envDefault:"postgres"` // 数据库类型：postgres / sqlite
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     int    `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" envDefault:"postgres"`
	Password string `env:"DB_PASSWORD"`
	DBName   string `env:"DB_NAME" envDefault:"chaos"`
	SSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`
	Path     string `env:"DB_PATH" envDefault:"chaos.db"` // sqlite 模式下的数据库文件路径
}

type PprofConfig struct {
	Enabled bool   `env:"PPROF_ENABLED" envDefault:"false"`
	Port    int    `env:"PPROF_PORT" envDefault:"6060"`
	Host    string `env:"PPROF_HOST" envDefault:"localhost"`
}

// SupabaseConfig 云端数据通道（Supabase Data API / PostgREST）配置。
// SecretKey 为后端专用凭据，只进 .env，禁止写入任何被版本控制的文件。
type SupabaseConfig struct {
	URL            string     `env:"SUPABASE_URL"`                         // 项目地址，形如 https://<project-ref>.supabase.co
	SecretKey      string     `env:"SUPABASE_SECRET_KEY"`                  // sb_secret_*：绕过 RLS，仅后端使用
	PublishableKey string     `env:"SUPABASE_PUBLISHABLE_KEY"`             // sb_publishable_*：受 RLS 约束，保留给将来的前端只读场景
	Schema         string     `env:"SUPABASE_SCHEMA" envDefault:"public"`  // 目标 schema，缺省 public
	TimeoutSec     int        `env:"SUPABASE_TIMEOUT_SEC" envDefault:"15"` // 单次请求超时秒数，缺省 15
	Tables         StringList `env:"SUPABASE_TABLES"`                      // 允许访问的表名清单（逗号分隔、自动去空白），空表示该通道不可用
}

// StringList 逗号分隔的字符串列表，解析时去除每项两端空白并丢弃空项。
// 用于 SUPABASE_TABLES 等场景，等价于原 parseCSV 的行为。
type StringList []string

func (s *StringList) UnmarshalText(text []byte) error {
	parts := strings.Split(string(text), ",")
	items := make(StringList, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	*s = items
	return nil
}

// Available 判定云端数据通道是否可用：项目地址、后端凭据与表名清单缺一不可。
func (c *SupabaseConfig) Available() bool {
	return c.URL != "" && c.SecretKey != "" && len(c.Tables) > 0
}

// MqttConfig 多节点消息同步通道（基于公共 MQTT broker）配置。
// Broker / Prefix 为公开信息；Username/Password 仅进 .env，禁止写入版本控制文件。
type MqttConfig struct {
	Enabled    bool   `env:"MQTT_ENABLED" envDefault:"false"`                    // 功能总开关，缺省 false
	Broker     string `env:"MQTT_BROKER" envDefault:"tcp://broker.emqx.io:1883"` // broker 地址
	Prefix     string `env:"MQTT_PREFIX" envDefault:"test/"`                     // 集群公共主题前缀，缺省 test/
	ClientID   string `env:"MQTT_CLIENT_ID"`                                     // 缺省 chaos-<nodeID>
	Username   string `env:"MQTT_USERNAME"`                                      // 公共 broker 多为匿名，留空
	Password   string `env:"MQTT_PASSWORD"`                                      // 同上
	Encrypt    bool   `env:"MQTT_ENCRYPT" envDefault:"false"`                    // 是否启用 AES-256-GCM 载荷加密
	EncryptKey string `env:"MQTT_ENCRYPT_KEY"`                                   // 32 字节共享密钥（hex 或 base64），仅进 .env，禁止入库/日志
}

var globalConfig *AppConfig

func LoadConfig() *AppConfig {
	if globalConfig != nil {
		return globalConfig
	}

	configDir := getConfigDir()
	configPath := filepath.Join(configDir, ".env")

	config := &AppConfig{}

	if _, err := os.Stat(configPath); err == nil {
		if err := godotenv.Load(configPath); err != nil {
			slog.Warn("无法加载配置文件", "path", configPath, "err", err)
		}
	} else {
		slog.Warn("配置文件不存在，使用默认配置", "path", configPath)
	}

	if err := envconfig.Parse(config); err != nil {
		slog.Warn("解析配置失败", "err", err)
	}

	// 日志通道：两者都未显式开启时，默认全部开启（兼容旧逻辑）
	if !config.LogToFile && !config.LogToConsole {
		config.LogToFile = true
		config.LogToConsole = true
	}

	globalConfig = config
	slog.Info("配置加载成功", "configDir", configDir)
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

func (c *PprofConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func InitLog() {
	cfg := GetConfig()
	if cfg == nil {
		return
	}

	var level slog.Level
	switch cfg.LogLevel {
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

	if cfg.LogToFile && cfg.LogFilePath != "" {
		dir := filepath.Dir(cfg.LogFilePath)
		os.MkdirAll(dir, 0o755)
		f, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Warn("failed to open log file, falling back to stderr only", "path", cfg.LogFilePath, "error", err)
		} else {
			slog.Info("log file configured", "path", cfg.LogFilePath)
			writers = append(writers, f)
		}
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
