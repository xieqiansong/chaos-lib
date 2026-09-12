package pagination

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type verifySnapshot struct {
	ID      int
	FileID  int
	Content string
}

func (verifySnapshot) TableName() string { return "quick_edit_snapshots" }

func tmpDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 user=test dbname=test sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func TestTmpRootCause(t *testing.T) {
	db := tmpDryRunDB(t)

	base := db.Where("file_id = ?", 5).Order("created_at DESC")
	var total int64
	errNoModel := base.Count(&total).Error
	t.Logf("无 Model 直接 Count -> %v", errNoModel)

	var snaps []verifySnapshot
	total, err := Paginate(db.Where("file_id = ?", 5).Order("created_at DESC"), &snaps, Query{Page: 2, Size: 20})
	t.Logf("Paginate -> total=%d err=%v", total, err)
	if err != nil {
		t.Fatalf("Paginate 仍失败: %v", err)
	}
	if errNoModel == nil || !strings.Contains(errNoModel.Error(), "Table not set") {
		t.Fatalf("未复现原始错误: %v", errNoModel)
	}
}
