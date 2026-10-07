// Command migrate 负责把 Postgres（源库）的数据全量迁移到 SQLite（目标库）。
//
// 设计要点：
//   - 复用全仓统一的 GORM 模型清单（与 internal/app/app.go 的 AutoMigrate 完全一致），
//     保证「建表结构」与「线上运行时」一致；
//   - 源库读取使用 Unscoped()，连同已被软删除（IsDeleted=1）的行一并搬走，保证数据完整性；
//   - 目标写入使用 SkipHooks 会话，避免任何钩子/插件干扰，原样落盘（含主键、软删标记、时间戳）；
//   - 迁移前默认清空目标表（硬删除，Unscoped），重复运行可安全幂等；
//   - 全仓模型不建真实外键约束，故无需按父子顺序写入。
//
// 用法示例：
//
//	# 用 configs/config.yaml 的 db 段拼接 Postgres DSN，目标写 chaos.db
//	go run ./cmd/migrate
//
//	# 显式指定源与目标
//	go run ./cmd/migrate --src "host=127.0.0.1 user=postgres password=secret dbname=chaos port=5432 sslmode=disable" --dst data/chaos.db
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"chaos-go/internal/domain/cronjob"
	"chaos-go/internal/domain/filelink"
	"chaos-go/internal/domain/note"
	"chaos-go/internal/domain/portfwd"
	"chaos-go/internal/domain/project"
	"chaos-go/internal/domain/proxy"
	"chaos-go/internal/domain/quickedit"
	"chaos-go/internal/domain/sdk"
	"chaos-go/internal/domain/standarddata"
	"chaos-go/internal/domain/taskplan"
	"chaos-go/internal/framework/apilog"
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/datacache"
	"chaos-go/internal/platform/mqttsync"
)

// models 是全仓需要迁移的模型清单，必须与 internal/app/app.go 中 AutoMigrate 的参数保持一致。
// 若日后新增表，请同时更新两处。
var models = []interface{}{
	&taskplan.TaskPlan{},
	&taskplan.Task{},
	&project.ProjectGroup{},
	&project.Project{},
	&proxy.BrowserHistory{},
	&proxy.BrowserHistoryVisit{},
	&proxy.Bookmark{},
	&portfwd.PortForwarding{},
	&portfwd.SshConnection{},
	&filelink.FileLink{},
	&quickedit.QuickEditFile{},
	&quickedit.QuickEditSnapshot{},
	&sdk.SdkSource{},
	&mqttsync.MqttSyncMessage{},
	&mqttsync.MqttSyncNode{},
	&cronjob.CronJob{},
	&cronjob.CronJobRun{},
	&standarddata.StandardData{},
	&datacache.DataCache{},
	&note.Note{},
	&note.NoteTag{},
	&note.NoteTagRel{},
	&note.NoteLink{},
	&apilog.ApiLog{},
}

func main() {
	srcDSN := flag.String("src", "", "Postgres libpq DSN；留空则用 configs/config.yaml 的 db 段拼接")
	dstPath := flag.String("dst", "", "目标 SQLite 文件路径；留空则用 db.path，再否则 chaos.db")
	batch := flag.Int("batch", 500, "每批读取/写入的行数")
	truncate := flag.Bool("truncate", true, "迁移前清空目标表（硬删除），保证可重复运行")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.LoadConfig()

	if *srcDSN == "" {
		*srcDSN = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
			cfg.Database.Host, cfg.Database.User, cfg.Database.Password,
			cfg.Database.DBName, cfg.Database.Port, cfg.Database.SSLMode,
		)
	}
	if *dstPath == "" {
		if cfg.Database.Path != "" {
			*dstPath = cfg.Database.Path
		} else {
			*dstPath = "chaos.db"
		}
	}

	slog.Info("开始迁移", "src", "postgres", "dst", *dstPath)

	src, err := openPostgres(*srcDSN)
	if err != nil {
		slog.Error("连接 Postgres 失败", "err", err)
		os.Exit(1)
	}
	dst, err := openSQLite(*dstPath)
	if err != nil {
		slog.Error("连接/初始化 SQLite 失败", "err", err)
		os.Exit(1)
	}

	if err := migrateAll(src, dst, *batch, *truncate); err != nil {
		slog.Error("迁移失败", "err", err)
		os.Exit(1)
	}

	slog.Info("迁移完成 ✅", "tables", len(models), "dst", *dstPath)
}

func openPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(8)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func openSQLite(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建 SQLite 目录失败: %w", err)
		}
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SQLite 写并发低：单写连接 + WAL
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		slog.Warn("设置 SQLite WAL 失败", "err", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		slog.Warn("开启 SQLite 外键失败", "err", err)
	}
	return db, nil
}

func tableName(db *gorm.DB, m interface{}) string {
	stmt := &gorm.Statement{DB: db}
	stmt.Parse(m)
	return stmt.Schema.Table
}

func migrateAll(src, dst *gorm.DB, batchSize int, truncate bool) error {
	slog.Info("在目标库创建/更新表结构")
	if err := config.AutoMigrate(dst, models...); err != nil {
		return fmt.Errorf("目标库迁移(建表)失败: %w", err)
	}

	for _, m := range models {
		if err := migrateOne(src, dst, m, batchSize, truncate); err != nil {
			return fmt.Errorf("迁移表 %s 失败: %w", tableName(dst, m), err)
		}
	}
	return nil
}

func migrateOne(src, dst *gorm.DB, model interface{}, batchSize int, truncate bool) error {
	name := tableName(dst, model)

	if truncate {
		// Unscoped 硬删除（即便是软删模型也会物理删除），便于重复运行幂等。
		if err := dst.Unscoped().Where("1 = 1").Delete(model).Error; err != nil {
			return fmt.Errorf("清空目标表失败: %w", err)
		}
	}

	sliceType := reflect.SliceOf(reflect.TypeOf(model))
	var offset int
	total := 0
	for {
		slicePtr := reflect.New(sliceType)
		sliceI := slicePtr.Interface()

		// Unscoped 读取：连同已软删除的行一并搬走，保证数据完整。
		res := src.Unscoped().Limit(batchSize).Offset(offset).Find(sliceI)
		if res.Error != nil {
			return fmt.Errorf("读取源表失败: %w", res.Error)
		}

		n := slicePtr.Elem().Len()
		if n == 0 {
			break
		}

		// SkipHooks：跳过钩子/插件，原样写入（保留主键、软删标记、时间戳）。
		if err := dst.Session(&gorm.Session{SkipHooks: true}).Create(sliceI).Error; err != nil {
			return fmt.Errorf("写入目标表失败: %w", err)
		}

		total += n
		offset += n
		if n < batchSize {
			break
		}
	}

	slog.Info("迁移表完成", "table", name, "rows", total)
	return nil
}
