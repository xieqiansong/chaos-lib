package config

import "gorm.io/gorm"

func SetDBForTest(db *gorm.DB) {
	dbInstance = db
}

func ResetDBForTest() {
	if dbInstance == nil {
		return
	}
	if sqlDB, err := dbInstance.DB(); err == nil {
		_ = sqlDB.Close()
	}
	dbInstance = nil
}
