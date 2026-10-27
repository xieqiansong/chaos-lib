package config

import "gorm.io/gorm"

// 本文件的两个方法只服务于测试：让测试能在「完全没有开发/生产数据库」的前提下，
// 把一个临时 sqlite 库塞进全局单例，从而让依赖 config.GetDB() 的业务代码跑到真实 SQL。
//
// 生产启动路径请勿调用它们：生产连接由 internal/framework/database 插件建立。

// SetDBForTest 注入测试专用连接；此后 GetDB() 直接返回它。
func SetDBForTest(db *gorm.DB) {
	SetDBProvider(func() *gorm.DB { return db })
}

// ResetDBForTest 关闭当前连接（若存在）并清空提供者，
// 使后续 GetDB() 返回 nil（生产路径下由 database 插件重新注入）。
func ResetDBForTest() {
	dbProviderMu.Lock()
	fn := dbProvider
	dbProvider = nil
	dbProviderMu.Unlock()
	if fn == nil {
		return
	}
	if db := fn(); db != nil {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}
