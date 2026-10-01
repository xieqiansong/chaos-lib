package quickedit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chaos-go/internal/pagination"
)

// maxContentLength 单文件内容上限（10MB）。
const maxContentLength = 10 * 1024 * 1024

// 领域错误哨兵：调用方据此映射 HTTP 状态码。
var (
	ErrFileExists      = errors.New("quickedit: 该文件已在管控列表中")
	ErrPathNotFound    = errors.New("quickedit: 文件路径不存在")
	ErrInvalidPath     = errors.New("quickedit: 路径无效")
	ErrContentTooLarge = errors.New("quickedit: 内容过大，暂不支持")
)

// ── 用例 ────────────────────────────────────────────────────────

// ListFilesWithSnapshot 列出全部受管控文件（含最近一次快照信息）。
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

// RegisterFile 登记受管控文件：校验路径存在、查重、读取内容并落首条快照。
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

// DeleteFileByID 删除受管控文件记录（不删磁盘文件）。
func DeleteFileByID(id int) error {
	if _, err := FindFileByID(id); err != nil {
		return err
	}
	if err := DeleteFile(id); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	return nil
}

// ReadContent 读取受管控文件的当前内容（虚拟文件走回调，其余读磁盘）。
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

// SaveContent 写入内容并追加一条快照（保存即版本化）。
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

// RestoreSnapshot 回滚到指定快照：写回快照内容并追加一条新快照。
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
		// 虚拟文件写入已生效，快照失败只降级为告警；磁盘文件则视为回滚未完成。
		if !isEnvVirtualFile(file.FilePath) {
			return nil, fmt.Errorf("文件已回滚，但快照失败: %w", snapErr)
		}
		warnings = append(warnings, "新快照失败: "+snapErr.Error())
	}
	return newSaveResult(file, afterSnap, snap.ID, warnings, "回滚成功", "环境变量已回滚"), nil
}

// ListSnapshotsOf 分页返回某文件的快照列表。
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

// GetSnapshot 读取单条快照内容。
func GetSnapshot(fileID, snapID int) (*QuickEditSnapshot, error) {
	if _, err := FindFileByID(fileID); err != nil {
		return nil, err
	}
	return FindSnapshot(fileID, snapID)
}

// newSaveResult 组装保存 / 回滚结果：刷新文件更新时间并富化响应。
// afterSnap 为 nil（快照失败且已降级为告警）时快照字段留零值。
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
