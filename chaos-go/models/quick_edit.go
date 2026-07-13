package models

import (
	"time"
)

type QuickEditFile struct {
	ID        int       `gorm:"primaryKey"`
	Name      string    ``
	FilePath  string    `gorm:"uniqueIndex:idx_quick_edit_files_path"`
	Remark    string    ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type QuickEditSnapshot struct {
	ID        int       `gorm:"primaryKey"`
	FileID    int       ``
	Content   string    ``
	SizeBytes int       ``
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type QuickEditFileResponse struct {
	ID               int       ``
	Name             string    ``
	FilePath         string    ``
	Remark           string    ``
	CreatedAt        time.Time ``
	UpdatedAt        time.Time ``
	LastSnapshotID   int       ``
	LastSnapshotTime time.Time ``
}

type QuickEditSnapshotResponse struct {
	ID        int       ``
	FileID    int       ``
	SizeBytes int       ``
	CreatedAt time.Time ``
}
