package envvar

import (
	"errors"
	"time"

	"chaos-go/internal/domain/quickedit"
	"gorm.io/gorm"
)

// 本文件是 envvar 包唯一的数据访问出口。
//
// 环境变量以 quickedit 的「虚拟文件」形式落库（快照即文件内容的历史版本），
// 因此这里不直接操作 quickedit 的表，一律经 quickedit 的 repository API，
// 避免跨模块直连 DB。系统环境变量本身的读写见 registry_windows.go / registry_other.go。

// ErrSnapshotNotFound 指定快照不存在（或不属于环境变量虚拟文件）。
var ErrSnapshotNotFound = errors.New("envvar: 快照不存在")

// ── 虚拟文件 ──────────────────────────────────────────────────

// findVirtualFile 按哨兵路径查找环境变量虚拟文件；未登记时返回 (nil, nil)。
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

// createVirtualFile 登记环境变量虚拟文件记录。
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

// ── 快照 ──────────────────────────────────────────────────────

// takeSnapshot 为虚拟文件追加一条内容快照。
func takeSnapshot(fileID int, content string) (*quickedit.QuickEditSnapshot, error) {
	return quickedit.TakeSnapshot(fileID, content)
}

// latestSnapshot 返回虚拟文件最近一次快照；尚未产生快照时返回 (nil, nil)。
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

// findSnapshot 按 (虚拟文件, 快照) 定位一条快照；不存在返回 ErrSnapshotNotFound。
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
