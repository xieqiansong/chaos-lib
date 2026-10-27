package config

import "gorm.io/gorm"

// 本文件的两个方法只服务于测试：让测试能在「完全没有开发/生产数据库」的前提下，
// 把一个临时 sqlite 库塞进全局单例，从而让依赖 config.GetDB() 的业务代码跑到真实 SQL。
//
// 生产启动路径请勿调用它们：main.go 通过 GetDB / TryConnectDB 自己建立连接。
// 之所以做成导出方法，是因为 crud / routes 等包的测试需要在包外完成注入。

// SetDBForTest 注入测试专用连接；此后 GetDB() 直接返回它。
func SetDBForTest(db *gorm.DB) {
	dbInstance = db
}

// ResetDBForTest 关闭当前连接（若存在）并把单例复位为未初始化状态，
// 使下一次 GetDB() 重新走 privateConnectDB 的正常建连路径。
func ResetDBForTest() {
	if dbInstance == nil {
		return
	}
	if sqlDB, err := dbInstance.DB(); err == nil {
		_ = sqlDB.Close()
	}
	dbInstance = nil
}
