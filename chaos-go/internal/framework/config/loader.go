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
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
	Path     string `yaml:"path"`
}

type PprofConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

type SupabaseConfig struct {
	URL            string     `yaml:"url"` // 项目地址，形如 https://<project-ref>.supabase.co
	SecretKey      string     `yaml:"secret_key"`
	PublishableKey string     `yaml:"publishable_key"`
	Schema         string     `yaml:"schema"`
	TimeoutSec     int        `yaml:"timeout_sec"`
	Tables         StringList `yaml:"tables"`
}

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

func (c *SupabaseConfig) Available() bool {
	return c.URL != "" && c.SecretKey != "" && len(c.Tables) > 0
}

type MqttConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Broker     string `yaml:"broker"`
	Prefix     string `yaml:"prefix"`
	ClientID   string `yaml:"client_id"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Encrypt    bool   `yaml:"encrypt"`
	EncryptKey string `yaml:"encrypt_key"`
}

type LogConfig struct {
	Level     string `yaml:"level"`
	FilePath  string `yaml:"file_path"`
	ToFile    bool   `yaml:"to_file"`
	ToConsole bool   `yaml:"to_console"`
}

type FeatureConfig struct {
	FileLink  bool `yaml:"file_link"`
	CronShell bool `yaml:"cron_shell"`
}

type DeepSeekConfig struct {
	APIKey string `yaml:"api_key"`
}

type BaiduConfig struct {
	AK string `yaml:"ak"`
}

type LuckyConfig struct {
	OpenToken       string `yaml:"open_token"`
	SyncIntervalSec int    `yaml:"sync_interval_sec"`
}

type StunConfig struct {
	PortForwardSyncIntervalSec int `yaml:"port_forward_sync_interval_sec"`
}

var globalConfig *AppConfig

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
