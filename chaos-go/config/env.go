package config

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	envconfig "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Environment string

const (
	EnvDev  Environment = "dev"
	EnvProd Environment = "prod"
)

type AppConfig struct {
	Environment Environment     `env:"-"`
	Server      ServerConfig    `envPrefix:"SERVER_"`
	Database    DatabaseConfig  `envPrefix:"DB_"`
	Pprof       PprofConfig     `envPrefix:"PPROF_"`
	Features    FeatureConfig   `envPrefix:"FEATURE_"`
	Log         LogConfig       `envPrefix:"LOG_"`
	DeepSeek    DeepSeekConfig  `envPrefix:"DEEPSEEK_"`
	Baidu       BaiduConfig     `envPrefix:"BAIDU_"`
	Supabase    SupabaseConfig  `envPrefix:"SUPABASE_"`
	Mqtt        MqttConfig      `envPrefix:"MQTT_"`
}

type ServerConfig struct {
	Port int    `env:"PORT" envDefault:"8080"`
	Host string `env:"HOST" envDefault:"0.0.0.0"`
}

type DatabaseConfig struct {
	Type     string `env:"TYPE" envDefault:"postgres"` // 数据库类型：postgres / sqlite
	Host     string `env:"HOST" envDefault:"localhost"`
	Port     int    `env:"PORT" envDefault:"5432"`
	User     string `env:"USER" envDefault:"postgres"`
	Password string `env:"PASSWORD"`
	DBName   string `env:"NAME" envDefault:"chaos"`
	SSLMode  string `env:"SSLMODE" envDefault:"disable"`
	Path     string `env:"PATH" envDefault:"chaos.db"` // sqlite 模式下的数据库文件路径
}

type PprofConfig struct {
	Enabled bool   `env:"ENABLED" envDefault:"false"`
	Port    int    `env:"PORT" envDefault:"6060"`
	Host    string `env:"HOST" envDefault:"localhost"`
}

type FeatureConfig struct {
	EnableFileLink bool `env:"FILE_LINK" envDefault:"true"`
}

type LogConfig struct {
	Level     string `env:"LEVEL" envDefault:"info"`
	FilePath  string `env:"FILE_PATH" envDefault:"logs/app.log"`
	ToFile    bool   `env:"TO_FILE"`
	ToConsole bool   `env:"TO_CONSOLE"`
}

type DeepSeekConfig struct {
	APIKey string `env:"API_KEY"`
}

type BaiduConfig struct {
	AK string `env:"AK"`
}

// SupabaseConfig 云端数据通道（Supabase Data API / PostgREST）配置。
// SecretKey 为后端专用凭据，只进 .env，禁止写入任何被版本控制的文件。
type SupabaseConfig struct {
	URL            string     `env:"URL"`             // 项目地址，形如 https://<project-ref>.supabase.co
	SecretKey      string     `env:"SECRET_KEY"`      // sb_secret_*：绕过 RLS，仅后端使用
	PublishableKey string     `env:"PUBLISHABLE_KEY"` // sb_publishable_*：受 RLS 约束，保留给将来的前端只读场景
	Schema         string     `env:"SCHEMA" envDefault:"public"`     // 目标 schema，缺省 public
	TimeoutSec     int        `env:"TIMEOUT_SEC" envDefault:"15"`    // 单次请求超时秒数，缺省 15
	Tables         StringList `env:"TABLES"`          // 允许访问的表名清单（逗号分隔、自动去空白），空表示该通道不可用
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
	Enabled    bool   `env:"ENABLED" envDefault:"false"`     // 功能总开关，缺省 false
	Broker     string `env:"BROKER" envDefault:"tcp://broker.emqx.io:1883"` // broker 地址
	Prefix     string `env:"PREFIX" envDefault:"test/"`      // 集群公共主题前缀，缺省 test/
	ClientID   string `env:"CLIENT_ID"`                      // 缺省 chaos-<nodeID>
	Username   string `env:"USERNAME"`                       // 公共 broker 多为匿名，留空
	Password   string `env:"PASSWORD"`                       // 同上
	Encrypt    bool   `env:"ENCRYPT" envDefault:"false"`     // 是否启用 AES-256-GCM 载荷加密
	EncryptKey string `env:"ENCRYPT_KEY"`                    // 32 字节共享密钥（hex 或 base64），仅进 .env，禁止入库/日志
}

var globalConfig *AppConfig

func LoadConfig() *AppConfig {
	if globalConfig != nil {
		return globalConfig
	}

	env := getEnvFromFlag()
	if env == "" {
		env = getEnvFromOS()
	}

	configDir := getConfigDir()
	configPath := resolveConfigPathFromDir(configDir, env)

	config := &AppConfig{}

	if configPath != "" {
		if err := godotenv.Load(configPath); err != nil {
			slog.Warn("无法加载配置文件", "path", configPath, "err", err)
		}
	}

	if err := envconfig.Parse(config); err != nil {
		slog.Warn("解析配置失败", "err", err)
	}

	config.Environment = env

	// 日志通道：两者都未显式开启时，默认全部开启（兼容旧逻辑）
	if !config.Log.ToFile && !config.Log.ToConsole {
		config.Log.ToFile = true
		config.Log.ToConsole = true
	}

	globalConfig = config
	slog.Info("配置加载成功", "env", env, "configDir", configDir)
	return config
}

func GetConfig() *AppConfig {
	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}

func getEnvFromOS() Environment {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	switch env {
	case "prod", "production":
		return EnvProd
	case "dev", "development":
		return EnvDev
	default:
		return EnvDev
	}
}

func getEnvFromFlag() Environment {
	envFlag := flag.String("env", "", "运行环境 (dev/prod)")
	flag.Parse()

	if *envFlag == "" {
		return ""
	}

	env := strings.ToLower(*envFlag)
	switch env {
	case "prod", "production":
		return EnvProd
	case "dev", "development":
		return EnvDev
	default:
		return Environment(env)
	}
}

func resolveConfigPathFromDir(dir string, env Environment) string {
	// 优先加载 .env
	defaultPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	// 其次加载 .env.{env}
	filename := fmt.Sprintf(".env.%s", env)
	path := filepath.Join(dir, filename)

	if _, err := os.Stat(path); err == nil {
		return path
	}

	slog.Warn("配置文件不存在，使用默认配置", "path", path)
	return ""
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
