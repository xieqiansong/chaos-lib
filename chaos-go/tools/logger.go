package tools

import (
	"io"
	"log/slog"
	"os"
	"time"
)

var Logger *slog.Logger

type LogConfig struct {
	Level     string
	FilePath  string
	ToFile    bool
	ToConsole bool
}

func InitLogger(cfg *LogConfig) {
	var writers []io.Writer

	if cfg.ToConsole {
		writers = append(writers, os.Stderr)
	}

	if cfg.ToFile {
		logPath := cfg.FilePath
		if !isAbs(logPath) {
			logPath = getLogPath(logPath)
		}

		logDir := getDir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			slog.Warn("创建日志目录失败", "err", err)
		} else {
			rotatedPath := rotateLogFileIfNeeded(logPath)
			file, err := os.OpenFile(rotatedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				slog.Warn("打开日志文件失败", "err", err)
			} else {
				writers = append(writers, file)
			}
		}
	}

	var handler slog.Handler
	if len(writers) > 0 {
		multiWriter := io.MultiWriter(writers...)
		handler = slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
			Level: getLevel(cfg.Level),
		})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: getLevel(cfg.Level),
		})
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

func getLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isAbs(path string) bool {
	for i := range path {
		if path[i] == '/' || path[i] == '\\' {
			return i == 0
		}
		if path[i] == ':' && i == 1 {
			return true
		}
	}
	return false
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func getLogPath(relativePath string) string {
	execPath, err := os.Executable()
	if err != nil {
		return relativePath
	}
	execDir := getDir(execPath)
	return execDir + string(os.PathSeparator) + relativePath
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

func Debug(msg string, args ...any) {
	if Logger != nil {
		Logger.Debug(msg, args...)
	} else {
		slog.Debug(msg, args...)
	}
}

func Info(msg string, args ...any) {
	if Logger != nil {
		Logger.Info(msg, args...)
	} else {
		slog.Info(msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if Logger != nil {
		Logger.Warn(msg, args...)
	} else {
		slog.Warn(msg, args...)
	}
}

func Error(msg string, args ...any) {
	if Logger != nil {
		Logger.Error(msg, args...)
	} else {
		slog.Error(msg, args...)
	}
}

func Debugf(format string, args ...any) {
	Debug(format, args...)
}

func Infof(format string, args ...any) {
	Info(format, args...)
}

func Warnf(format string, args ...any) {
	Warn(format, args...)
}

func Errorf(format string, args ...any) {
	Error(format, args...)
}
