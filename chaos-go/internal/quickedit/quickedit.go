package quickedit

import (
	"chaos-go/config"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

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
	ID               int
	Name             string
	FilePath         string
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastSnapshotID   int
	LastSnapshotTime time.Time
}

type QuickEditSnapshotResponse struct {
	ID        int
	FileID    int
	SizeBytes int
	CreatedAt time.Time
}

// ── 环境变量虚拟文件常量 ─────────────────────────────────────────

const (
	EnvVirtualFilePath = "__env__all__"
	EnvVirtualFileName = "环境变量"
	EnvVirtualRemark   = "系统 + 用户环境变量快照（TOML 格式）"
)

// EnvReader 读取环境变量内容（回调 — 避免循环依赖 envvar）
// 由 main.go 在启动时注入 envvar.ReadVirtualContent
var (
	EnvReadContent  func() (content string, sizeBytes int, err error)
	EnvWriteContent func(content string) (warnings []string, err error)
)

func isEnvVirtualFile(filePath string) bool {
	return filePath == EnvVirtualFilePath
}

// ── 数据访问辅助 ──────────────────────────────────────────────────

func TakeSnapshot(fileID int, content string) (*QuickEditSnapshot, error) {
	snapshot := QuickEditSnapshot{
		FileID:    fileID,
		Content:   content,
		SizeBytes: len([]byte(content)),
		CreatedAt: time.Now(),
	}
	if err := config.GetDB().Create(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func findFileByID(c *gin.Context, id int) (*QuickEditFile, bool) {
	var file QuickEditFile
	if err := config.GetDB().First(&file, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return nil, false
	}
	return &file, true
}

func buildFileResponse(file QuickEditFile) QuickEditFileResponse {
	resp := QuickEditFileResponse{
		ID:        file.ID,
		Name:      file.Name,
		FilePath:  file.FilePath,
		Remark:    file.Remark,
		CreatedAt: file.CreatedAt,
		UpdatedAt: file.UpdatedAt,
	}
	var latest QuickEditSnapshot
	config.GetDB().Where("file_id = ?", file.ID).Order("created_at DESC, id DESC").First(&latest)
	if latest.ID > 0 {
		resp.LastSnapshotID = latest.ID
		resp.LastSnapshotTime = latest.CreatedAt
	}
	return resp
}

// ── Handlers ──────────────────────────────────────────────────────

const maxContentLength = 10 * 1024 * 1024

func ListQuickEdits(c *gin.Context) {
	var files []QuickEditFile
	if err := config.GetDB().Order("created_at DESC, id DESC").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	responses := make([]QuickEditFileResponse, 0, len(files))
	for _, f := range files {
		responses = append(responses, buildFileResponse(f))
	}
	c.JSON(http.StatusOK, responses)
}

func CreateQuickEdit(c *gin.Context) {
	var req struct {
		Name     string
		FilePath string
		Remark   string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	absPath, err := filepath.Abs(req.FilePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径无效: " + err.Error()})
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不存在"})
		return
	}
	var existing QuickEditFile
	if err := config.GetDB().Where("file_path = ?", absPath).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该文件已在管控列表中"})
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}
	if len(data) > maxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件过大，暂不支持"})
		return
	}
	name := req.Name
	if name == "" {
		name = filepath.Base(absPath)
	}
	file := QuickEditFile{
		Name:      name,
		FilePath:  absPath,
		Remark:    req.Remark,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := config.GetDB().Create(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	snapshot, err := TakeSnapshot(file.ID, string(data))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件已登记，但快照失败: " + err.Error()})
		return
	}
	resp := buildFileResponse(file)
	c.JSON(http.StatusOK, gin.H{
		"message":         "创建成功",
		"data":            resp,
		"firstSnapshotId": snapshot.ID,
	})
}

func DeleteQuickEdit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var file QuickEditFile
	if err := config.GetDB().First(&file, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	if err := config.GetDB().Delete(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func GetQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	file, ok := findFileByID(c, id)
	if !ok {
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvReadContent == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "EnvReadContent 回调未初始化"})
			return
		}
		content, size, err := EnvReadContent()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取环境变量失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": content, "filePath": file.FilePath, "sizeBytes": size})
		return
	}
	data, err := os.ReadFile(file.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(data), "filePath": file.FilePath, "sizeBytes": len(data)})
}

func UpdateQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	file, ok := findFileByID(c, id)
	if !ok {
		return
	}
	var req struct{ Content string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len([]byte(req.Content)) > maxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容过大，暂不支持"})
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvWriteContent == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "EnvWriteContent 回调未初始化"})
			return
		}
		warnings, writeErr := EnvWriteContent(req.Content)
		if writeErr != nil {
			warnings = append(warnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnap, snapErr := TakeSnapshot(file.ID, req.Content)
		if snapErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "环境变量已更新，但快照失败: " + snapErr.Error(), "warnings": warnings})
			return
		}
		file.UpdatedAt = time.Now()
		config.GetDB().Model(&QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
		resp := buildFileResponse(*file)
		c.JSON(http.StatusOK, gin.H{"message": "环境变量已更新", "data": resp, "snapshotId": afterSnap.ID, "snapshotTime": afterSnap.CreatedAt, "warnings": warnings})
		return
	}
	if err := os.WriteFile(file.FilePath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败 (可能需要管理员权限): " + err.Error()})
		return
	}
	afterSnap, snapErr := TakeSnapshot(file.ID, req.Content)
	if snapErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件已更新，但快照失败: " + snapErr.Error()})
		return
	}
	file.UpdatedAt = time.Now()
	config.GetDB().Model(&QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
	resp := buildFileResponse(*file)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "data": resp, "snapshotId": afterSnap.ID, "snapshotTime": afterSnap.CreatedAt})
}

func ListQuickEditSnapshots(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	if _, ok := findFileByID(c, id); !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	var total int64
	config.GetDB().Model(&QuickEditSnapshot{}).Where("file_id = ?", id).Count(&total)
	var snaps []QuickEditSnapshot
	if err := config.GetDB().Where("file_id = ?", id).Order("created_at DESC, id DESC").Limit(size).Offset((page - 1) * size).Find(&snaps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	items := make([]QuickEditSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		items = append(items, QuickEditSnapshotResponse{ID: s.ID, FileID: s.FileID, SizeBytes: s.SizeBytes, CreatedAt: s.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

func GetQuickEditSnapshot(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 file id"})
		return
	}
	snapID, err := strconv.Atoi(c.Param("snapshotId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 snapshot id"})
		return
	}
	if _, ok := findFileByID(c, fileID); !ok {
		return
	}
	var snap QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": snap.ID, "fileId": snap.FileID, "content": snap.Content, "sizeBytes": snap.SizeBytes, "createdAt": snap.CreatedAt})
}

func RestoreQuickEdit(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	file, ok := findFileByID(c, fileID)
	if !ok {
		return
	}
	var req struct{ SnapshotID int }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var snap QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", req.SnapshotID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvWriteContent == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "EnvWriteContent 回调未初始化"})
			return
		}
		writeWarnings, writeErr := EnvWriteContent(snap.Content)
		if writeErr != nil {
			writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnap, snapErr := TakeSnapshot(fileID, snap.Content)
		if snapErr != nil {
			writeWarnings = append(writeWarnings, "新快照失败: "+snapErr.Error())
		}
		file.UpdatedAt = time.Now()
		config.GetDB().Model(&QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
		resp := buildFileResponse(*file)
		c.JSON(http.StatusOK, gin.H{"message": "环境变量已回滚", "data": resp, "fromSnapshotId": snap.ID, "snapshotId": afterSnap.ID, "warnings": writeWarnings})
		return
	}
	if err := os.WriteFile(file.FilePath, []byte(snap.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败 (可能需要管理员权限): " + err.Error()})
		return
	}
	afterSnap, snapErr := TakeSnapshot(fileID, snap.Content)
	if snapErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件已回滚，但快照失败: " + snapErr.Error()})
		return
	}
	file.UpdatedAt = time.Now()
	config.GetDB().Model(&QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
	resp := buildFileResponse(*file)
	c.JSON(http.StatusOK, gin.H{"message": "回滚成功", "data": resp, "fromSnapshotId": snap.ID, "snapshotId": afterSnap.ID})
}
