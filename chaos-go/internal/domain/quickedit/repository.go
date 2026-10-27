package quickedit

import (
	"errors"
	"time"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/pagination"
	"gorm.io/gorm"
)

func TakeSnapshot(fileID int, content string) (*QuickEditSnapshot, error) {
	snapshot := QuickEditSnapshot{
		FileID:    fileID,
		Content:   content,
		SizeBytes: len([]byte(content)),
		CreatedAt: time.Now(),
	}
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	if err := db.Create(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func FindFileByID(id int) (*QuickEditFile, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var file QuickEditFile
	if err := db.First(&file, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return &file, nil
}

func GetLatestSnapshot(fileID int) (*QuickEditSnapshot, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var latest QuickEditSnapshot
	err := db.Where("file_id = ?", fileID).Order("created_at DESC, id DESC").First(&latest).Error
	if err != nil {
		return nil, err
	}
	return &latest, nil
}

func ListFiles() ([]QuickEditFile, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var files []QuickEditFile
	if err := db.Order("created_at DESC, id DESC").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func FindFileByPath(absPath string) (*QuickEditFile, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var existing QuickEditFile
	if err := db.Where("file_path = ?", absPath).First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func CreateFile(file *QuickEditFile) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(file).Error
}

func DeleteFile(id int) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Delete(&QuickEditFile{}, "id = ?", id).Error
}

func UpdateFileUpdatedAt(id int, t time.Time) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&QuickEditFile{}).Where("id = ?", id).Update("updated_at", t).Error
}

func ListSnapshots(fileID int, q pagination.Query) ([]QuickEditSnapshot, int64, error) {
	db := config.GetDB()
	if db == nil {
		return nil, 0, ErrDBUnavailable
	}
	var snaps []QuickEditSnapshot
	total, err := pagination.Paginate(
		db.Where("file_id = ?", fileID).Order("created_at DESC, id DESC"),
		&snaps,
		q,
	)
	if err != nil {
		return nil, 0, err
	}
	return snaps, total, nil
}

func FindSnapshot(fileID, snapID int) (*QuickEditSnapshot, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var snap QuickEditSnapshot
	if err := db.Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}
	return &snap, nil
}
