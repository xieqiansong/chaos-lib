// Package database 把数据库连接做成一个插件：连接、重试、自动迁移与就绪通知都由这里统一负责。
//
// 设计要点：
//   - 连接失败不阻断启动：Start 只做有限等待，随后在后台按退避策略持续重试；
//   - 连接成功后执行一次 AutoMigrate（模型清单由装配层通过 SetModels 注入，含框架模型）；
//   - 依赖数据库的插件通过 OnReady 在连接成功后补做初始化（如 cronjob 重新加载用户任务）；
//   - 业务代码仍统一用 config.GetDB() 读取连接，连接尚未建立时返回 nil，由接口层报错。
package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/plugin"
)

const (
	// startupWait 是 Start 阶段等待连接的上限：连上则后续插件可直接用库；
	// 超时则转入后台重试，应用以降级模式继续启动。
	startupWait  = 3 * time.Second
	initialRetry = 5 * time.Second
	maxRetry     = time.Minute
)

var (
	mu          sync.RWMutex
	conn        *gorm.DB
	ready       bool
	connectedAt time.Time
	models      []any
	onReady     []func()
)

func init() {
	config.SetDBProvider(DB)
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{ID: "database", Name: "数据库", Version: "1.0.0"},
		Start: func() error {
			go connectLoop()
			deadline := time.Now().Add(startupWait)
			for time.Now().Before(deadline) {
				if Ready() {
					return nil
				}
				time.Sleep(50 * time.Millisecond)
			}
			slog.Warn("数据库未在启动等待窗口内就绪，转入后台重试", "wait", startupWait)
			return nil
		},
	})
}

// SetModels 注入需要 AutoMigrate 的模型清单。必须在插件 Start 之前调用。
func SetModels(m []any) {
	mu.Lock()
	defer mu.Unlock()
	models = m
}

// DB 返回当前连接；尚未连上时为 nil。
func DB() *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()
	return conn
}

// Ready 表示数据库是否已连接成功。
func Ready() bool {
	mu.RLock()
	defer mu.RUnlock()
	return ready
}

// OnReady 注册连接成功后的回调；若当前已就绪则异步立即执行。
// 用于依赖数据库的插件在「启动期库不可用、稍后恢复」时补做初始化。
func OnReady(fn func()) {
	mu.Lock()
	if ready {
		mu.Unlock()
		go fn()
		return
	}
	onReady = append(onReady, fn)
	mu.Unlock()
}

// connectLoop 持续重试直到连接成功；成功后执行迁移并通知等待方。
func connectLoop() {
	delay := initialRetry
	for {
		db, err := open()
		if err != nil {
			slog.Warn("数据库连接失败，稍后重试", "err", err, "retryIn", delay)
			time.Sleep(delay)
			if delay < maxRetry {
				delay *= 2
				if delay > maxRetry {
					delay = maxRetry
				}
			}
			continue
		}

		mu.Lock()
		conn = db
		ready = true
		connectedAt = time.Now()
		pending := onReady
		onReady = nil
		migrateModels := models
		mu.Unlock()

		slog.Info("数据库连接成功", "at", connectedAt)
		if err := migrate(db, migrateModels); err != nil {
			slog.Error("数据库迁移失败", "err", err)
		}

		for _, fn := range pending {
			fn()
		}
		return
	}
}

// migrate 建表 / 补列。迁移失败只告警，不阻断后续启动。
func migrate(db *gorm.DB, list []any) error {
	if len(list) == 0 {
		return nil
	}
	if err := config.AutoMigrate(db, list...); err != nil {
		return err
	}
	slog.Info("数据库自动迁移完成", "models", len(list))
	return nil
}

// open 按配置建立连接并设置连接池参数。
func open() (*gorm.DB, error) {
	cfg := config.GetConfig()
	dsn := cfg.Database.GetDSN()

	var dialector gorm.Dialector
	switch cfg.Database.Type {
	case "sqlite":
		// 确保数据库文件所在目录存在
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("创建 sqlite 目录失败: %w", err)
			}
		}
		dialector = sqlite.Open(dsn)
	default:
		dialector = postgres.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 sql.DB 失败: %w", err)
	}

	if cfg.Database.Type == "sqlite" {
		// SQLite 写并发低：单写连接 + WAL，避免 database is locked
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			slog.Warn("设置 SQLite WAL 模式失败", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
			slog.Warn("开启 SQLite 外键失败", "err", err)
		}
	} else {
		sqlDB.SetMaxOpenConns(32)
		sqlDB.SetMaxIdleConns(8)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	return db, nil
}
