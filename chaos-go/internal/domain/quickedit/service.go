package quickedit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chaos-go/internal/framework/pagination"
)

const maxContentLength = 10 * 1024 * 1024

var (
	ErrFileExists      = errors.New("quickedit: 该文件已在管控列表中")
	ErrPathNotFound    = errors.New("quickedit: 文件路径不存在")
	ErrInvalidPath     = errors.New("quickedit: 路径无效")
	ErrContentTooLarge = errors.New("quickedit: 内容过大，暂不支持")
)

func ListFilesWithSnapshot() ([]QuickEditFileResponse, error) {
	files, err := ListFiles()
	if err != nil {
		return nil, err
	}
	items := make([]QuickEditFileResponse, 0, len(files))
	for _, f := range files {
		items = append(items, buildFileResponse(f))
	}
	return items, nil
}

func RegisterFile(name, rawPath, remark string) (*QuickEditFile, *QuickEditSnapshot, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, nil, ErrPathNotFound
	}
	if _, err := FindFileByPath(absPath); err == nil {
		return nil, nil, ErrFileExists
	}
	data, err := readDiskFile(absPath)
	if err != nil {
		return nil, nil, err
	}
	if len(data) > maxContentLength {
		return nil, nil, ErrContentTooLarge
	}
	if name == "" {
		name = filepath.Base(absPath)
	}
	now := time.Now()
	file := QuickEditFile{
		Name:      name,
		FilePath:  absPath,
		Remark:    remark,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := CreateFile(&file); err != nil {
		return nil, nil, fmt.Errorf("创建失败: %w", err)
	}
	snap, err := TakeSnapshot(file.ID, string(data))
	if err != nil {
		return nil, nil, fmt.Errorf("文件已登记，但快照失败: %w", err)
	}
	return &file, snap, nil
}

func DeleteFileByID(id int) error {
	if _, err := FindFileByID(id); err != nil {
		return err
	}
	if err := DeleteFile(id); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	return nil
}

func ReadContent(id int) (*ContentView, error) {
	file, err := FindFileByID(id)
	if err != nil {
		return nil, err
	}
	content, size, err := readContent(file)
	if err != nil {
		return nil, err
	}
	return &ContentView{Content: content, FilePath: file.FilePath, SizeBytes: size}, nil
}

func SaveContent(id int, content string) (*SaveResult, error) {
	file, err := FindFileByID(id)
	if err != nil {
		return nil, err
	}
	if len([]byte(content)) > maxContentLength {
		return nil, ErrContentTooLarge
	}
	warnings, err := writeContent(file, content)
	if err != nil {
		return nil, err
	}
	snap, err := TakeSnapshot(file.ID, content)
	if err != nil {
		if isEnvVirtualFile(file.FilePath) {
			return nil, fmt.Errorf("环境变量已更新，但快照失败: %w", err)
		}
		return nil, fmt.Errorf("文件已更新，但快照失败: %w", err)
	}
	return newSaveResult(file, snap, 0, warnings, "更新成功", "环境变量已更新"), nil
}

func RestoreSnapshot(fileID, snapID int) (*SaveResult, error) {
	file, err := FindFileByID(fileID)
	if err != nil {
		return nil, err
	}
	snap, err := FindSnapshot(fileID, snapID)
	if err != nil {
		return nil, err
	}
	warnings, err := writeContent(file, snap.Content)
	if err != nil {
		return nil, err
	}
	afterSnap, snapErr := TakeSnapshot(fileID, snap.Content)
	if snapErr != nil {

		if !isEnvVirtualFile(file.FilePath) {
			return nil, fmt.Errorf("文件已回滚，但快照失败: %w", snapErr)
		}
		warnings = append(warnings, "新快照失败: "+snapErr.Error())
	}
	return newSaveResult(file, afterSnap, snap.ID, warnings, "回滚成功", "环境变量已回滚"), nil
}

func ListSnapshotsOf(fileID int, q pagination.Query) ([]QuickEditSnapshotResponse, int64, error) {
	if _, err := FindFileByID(fileID); err != nil {
		return nil, 0, err
	}
	snaps, total, err := ListSnapshots(fileID, q)
	if err != nil {
		return nil, 0, err
	}
	return toSnapshotResponses(snaps), total, nil
}

func GetSnapshot(fileID, snapID int) (*QuickEditSnapshot, error) {
	if _, err := FindFileByID(fileID); err != nil {
		return nil, err
	}
	return FindSnapshot(fileID, snapID)
}

func newSaveResult(file *QuickEditFile, afterSnap *QuickEditSnapshot, fromSnapID int,
	warnings []string, msg, envMsg string) *SaveResult {
	file.UpdatedAt = time.Now()
	_ = UpdateFileUpdatedAt(file.ID, file.UpdatedAt)
	view := buildFileResponse(*file)
	message := msg
	if isEnvVirtualFile(file.FilePath) {
		message = envMsg
	}
	res := &SaveResult{
		Message:        message,
		Data:           &view,
		FromSnapshotID: fromSnapID,
		Warnings:       warnings,
	}
	if afterSnap != nil {
		res.SnapshotID = afterSnap.ID
		res.SnapshotTime = afterSnap.CreatedAt
	}
	return res
}
