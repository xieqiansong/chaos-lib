package envvar

import (
	"errors"
	"time"

	"chaos-go/internal/domain/quickedit"
	"gorm.io/gorm"
)

var ErrSnapshotNotFound = errors.New("envvar: 快照不存在")

func findVirtualFile() (*quickedit.QuickEditFile, error) {
	file, err := quickedit.FindFileByPath(quickedit.EnvVirtualFilePath)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return file, nil
}

func createVirtualFile() (*quickedit.QuickEditFile, error) {
	now := time.Now()
	file := &quickedit.QuickEditFile{
		Name:      quickedit.EnvVirtualFileName,
		FilePath:  quickedit.EnvVirtualFilePath,
		Remark:    quickedit.EnvVirtualRemark,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := quickedit.CreateFile(file); err != nil {
		return nil, err
	}
	return file, nil
}

func takeSnapshot(fileID int, content string) (*quickedit.QuickEditSnapshot, error) {
	return quickedit.TakeSnapshot(fileID, content)
}

func latestSnapshot(fileID int) (*quickedit.QuickEditSnapshot, error) {
	snap, err := quickedit.GetLatestSnapshot(fileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if snap == nil || snap.ID == 0 {
		return nil, nil
	}
	return snap, nil
}

func findSnapshot(fileID, snapID int) (*quickedit.QuickEditSnapshot, error) {
	snap, err := quickedit.FindSnapshot(fileID, snapID)
	if err != nil {
		if errors.Is(err, quickedit.ErrSnapshotNotFound) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}
	return snap, nil
}
