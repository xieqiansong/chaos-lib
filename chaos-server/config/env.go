package config

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	EnvDev  Environment = "dev"
	EnvProd Environment = "prod"
)

type AppConfig struct {
	Environment Environment
	Server      ServerConfig
	Database    DatabaseConfig
	Pprof       PprofConfig
	Features    FeatureConfig
	Log         LogConfig
	DeepSeek    DeepSeekConfig
}

type ServerConfig struct {
	Port int
	Host string
}

type DatabaseConfig struct {
	Type     string // 数据库类型：postgres / sqlite，默认 postgres
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	Path     string // sqlite 模式下的数据库文件路径，默认 chaos.db
}

type PprofConfig struct {
	Enabled bool
	Port    int
	Host    string
}

type FeatureConfig struct {
	EnableFileLink bool
}

type LogConfig struct {
	Level     string
	FilePath  string
	ToFile    bool
	ToConsole bool
}

type DeepSeekConfig struct {
	APIKey string
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

	config := &AppConfig{
		Environment: env,
	}

	loadConfigFile(config, configPath)
	setDefaults(config)

	globalConfig = config
	log.Printf("✅ 配置加载成功 [环境: %s] [配置目录: %s]", env, configDir)
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

	log.Printf("️ 配置文件不存在: %s，使用默认配置", path)
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

func loadConfigFile(config *AppConfig, path string) {
	if path == "" {
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("⚠️ 无法打开配置文件: %v", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		setConfigValue(config, key, value)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ 读取配置文件错误: %v", err)
	}
}

func setConfigValue(config *AppConfig, key, value string) {
	switch key {
	case "SERVER_PORT":
		if port, err := strconv.Atoi(value); err == nil {
			config.Server.Port = port
		}
	case "SERVER_HOST":
		config.Server.Host = value
	case "DB_HOST":
		config.Database.Host = value
	case "DB_PORT":
		if port, err := strconv.Atoi(value); err == nil {
			config.Database.Port = port
		}
	case "DB_USER":
		config.Database.User = value
	case "DB_PASSWORD":
		config.Database.Password = value
	case "DB_NAME":
		config.Database.DBName = value
	case "DB_SSLMODE":
		config.Database.SSLMode = value
	case "DB_TYPE":
		config.Database.Type = strings.ToLower(value)
	case "DB_PATH":
		config.Database.Path = value
	case "PPROF_ENABLED":
		config.Pprof.Enabled = parseBool(value)
	case "PPROF_PORT":
		if port, err := strconv.Atoi(value); err == nil {
			config.Pprof.Port = port
		}
	case "PPROF_HOST":
		config.Pprof.Host = value
	case "FEATURE_FILE_LINK":
		config.Features.EnableFileLink = parseBool(value)
	case "LOG_LEVEL":
		config.Log.Level = value
	case "LOG_FILE_PATH":
		config.Log.FilePath = value
	case "LOG_TO_FILE":
		config.Log.ToFile = parseBool(value)
	case "LOG_TO_CONSOLE":
		config.Log.ToConsole = parseBool(value)
	case "DEEPSEEK_API_KEY":
		config.DeepSeek.APIKey = value
	}
}

func setDefaults(config *AppConfig) {
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Database.Host == "" {
		config.Database.Host = "localhost"
	}
	if config.Database.Port == 0 {
		config.Database.Port = 5432
	}
	if config.Database.User == "" {
		config.Database.User = "postgres"
	}
	if config.Database.DBName == "" {
		config.Database.DBName = "chaos"
	}
	if config.Database.SSLMode == "" {
		config.Database.SSLMode = "disable"
	}
	if config.Database.Type == "" {
		config.Database.Type = "postgres"
	}
	if config.Database.Path == "" {
		config.Database.Path = "chaos.db"
	}
	if config.Pprof.Port == 0 {
		config.Pprof.Port = 6060
	}
	if config.Pprof.Host == "" {
		config.Pprof.Host = "localhost"
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.FilePath == "" {
		config.Log.FilePath = "logs/app.log"
	}
	if !config.Log.ToFile && !config.Log.ToConsole {
		config.Log.ToFile = true
		config.Log.ToConsole = true
	}

	config.Features.EnableFileLink = true
}

func parseBool(value string) bool {
	lower := strings.ToLower(value)
	return lower == "true" || lower == "1" || lower == "yes" || lower == "on"
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

	var writers []io.Writer

	if cfg.Log.ToConsole {
		writers = append(writers, os.Stderr)
	}

	if cfg.Log.ToFile {
		logPath := cfg.Log.FilePath
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(getConfigDir(), logPath)
		}

		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("⚠️ 创建日志目录失败: %v", err)
		} else {
			rotatedPath := rotateLogFileIfNeeded(logPath)
			file, err := os.OpenFile(rotatedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("⚠️ 打开日志文件失败: %v", err)
			} else {
				writers = append(writers, file)
			}
		}
	}

	if len(writers) > 0 {
		multiWriter := io.MultiWriter(writers...)
		log.SetOutput(multiWriter)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}

func rotateLogFileIfNeeded(logPath string) string {
	if _, err := os.Stat(logPath); err != nil {
		return logPath
	}

	info, err := os.Stat(logPath)
	if err != nil {
		return logPath
	}

	now := time.Now()
	modTime := info.ModTime()
	if now.YearDay() != modTime.YearDay() || now.Year() != modTime.Year() {
		backup := logPath + "." + modTime.Format("2006-01-02")
		_ = os.Rename(logPath, backup)
	}

	return logPath
}
