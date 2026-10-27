package config

import (
	"sync"

	"gorm.io/gorm"
)

// dbProvider 由 database 插件在 init 阶段注入（见 internal/framework/database），
// 使 config 不感知连接细节、也不反向依赖插件包。
var (
	dbProviderMu sync.RWMutex
	dbProvider   func() *gorm.DB
)

// SetDBProvider 注入数据库连接提供者。由 database 插件调用，业务代码不直接使用。
func SetDBProvider(fn func() *gorm.DB) {
	dbProviderMu.Lock()
	defer dbProviderMu.Unlock()
	dbProvider = fn
}

// AutoMigrate 通用数据库迁移。由 database 插件与 cmd/migrate 调用，
// 传入所有需要建表的模型实例。
func AutoMigrate(db *gorm.DB, dst ...interface{}) error {
	return db.AutoMigrate(dst...)
}

// GetDB 返回当前可用的数据库连接；database 插件尚未连上时返回 nil。
// 调用方需自行判空：数据库不可用属于降级状态，由接口层返回错误。
func GetDB() *gorm.DB {
	dbProviderMu.RLock()
	fn := dbProvider
	dbProviderMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn()
}
