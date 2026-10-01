package quickedit

import (
	"time"

	"chaos-go/internal/config"
	"chaos-go/internal/pagination"
)

// TakeSnapshot 写入一条内容快照（供本包与 envvar 共用）。
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

// FindFileByID 按 ID 加载文件记录。
func FindFileByID(id int) (*QuickEditFile, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var file QuickEditFile
	if err := db.First(&file, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// GetLatestSnapshot 返回某文件最近一次快照。
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

// ListFiles 返回全部文件（按创建时间倒序）。
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

// FindFileByPath 按绝对路径查重。
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

// CreateFile 登记一个新文件。
func CreateFile(file *QuickEditFile) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Create(file).Error
}

// DeleteFile 物理删除某文件记录。
func DeleteFile(id int) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Delete(&QuickEditFile{}, "id = ?", id).Error
}

// UpdateFileUpdatedAt 刷新文件更新时间。
func UpdateFileUpdatedAt(id int, t time.Time) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Model(&QuickEditFile{}).Where("id = ?", id).Update("updated_at", t).Error
}

// ListSnapshots 分页返回某文件的快照列表（按时间倒序）。
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

// FindSnapshot 按 (fileID, snapID) 加载单条快照。
func FindSnapshot(fileID, snapID int) (*QuickEditSnapshot, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var snap QuickEditSnapshot
	if err := db.Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}
