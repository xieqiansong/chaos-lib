package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"chaos-lib/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dbInstance *gorm.DB

func privateConnectDB() (*gorm.DB, error) {
	cfg := GetConfig()
	dsn := cfg.Database.GetDSN()

	var dialector gorm.Dialector
	switch cfg.Database.Type {
	case "sqlite":
		// 确保数据库文件所在目录存在
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create sqlite dir: %v", err)
			}
		}
		dialector = sqlite.Open(dsn)
	default:
		dialector = postgres.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB from GORM: %v", err)
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

	// 自动迁移：首次启动建表，已存在则幂等（不会破坏现有数据）
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %v", err)
	}

	return db, nil
}

// autoMigrate 依据模型结构创建/更新表，使 PostgreSQL 与 SQLite 共用同一份表定义。
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.BrowserHistory{},
		&models.BrowserHistoryVisit{},
		&models.PortForwarding{},
		&models.FileLink{},
		&models.QuickEditFile{},
		&models.QuickEditSnapshot{},
		&models.ProjectGroup{},
		&models.Project{},
		&models.TaskPlan{},
		&models.Task{},
	)
}

func GetDB() *gorm.DB {
	if dbInstance == nil {
		db, err := privateConnectDB()
		if err != nil {
			slog.Error("数据库连接失败", "err", err)
			return nil
		}
		dbInstance = db
		slog.Info("Connected to database with GORM")
	}
	return dbInstance
}

func TryConnectDB() (*gorm.DB, error) {
	if dbInstance != nil {
		return dbInstance, nil
	}

	db, err := privateConnectDB()
	if err != nil {
		return nil, err
	}

	dbInstance = db
	slog.Info("Connected to database with GORM")
	return db, nil
}
