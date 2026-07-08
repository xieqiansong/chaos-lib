package services

import (
	"chaos-lib/config"
	"chaos-lib/models"
	"chaos-lib/tools"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func isEnvVirtualFile(filePath string) bool {
	return filePath == models.EnvVirtualFilePath
}

const maxContentLength = 10 * 1024 * 1024

func takeSnapshot(fileID int, content string) (*models.QuickEditSnapshot, error) {
	snapshot := models.QuickEditSnapshot{
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

func findFileByID(c *gin.Context, id int) (*models.QuickEditFile, bool) {
	var file models.QuickEditFile
	if err := config.GetDB().First(&file, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return nil, false
	}
	return &file, true
}

func buildFileResponse(file models.QuickEditFile) models.QuickEditFileResponse {
	resp := models.QuickEditFileResponse{
		ID:        file.ID,
		Name:      file.Name,
		FilePath:  file.FilePath,
		Remark:    file.Remark,
		CreatedAt: file.CreatedAt,
		UpdatedAt: file.UpdatedAt,
	}

	var latest models.QuickEditSnapshot
	config.GetDB().Where("file_id = ?", file.ID).Order("created_at DESC, id DESC").First(&latest)
	if latest.ID > 0 {
		resp.LastSnapshotID = latest.ID
		resp.LastSnapshotTime = latest.CreatedAt
	}

	return resp
}

func ListQuickEdits(c *gin.Context) {
	var files []models.QuickEditFile
	if err := config.GetDB().Order("created_at DESC, id DESC").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	responses := make([]models.QuickEditFileResponse, 0, len(files))
	for _, f := range files {
		responses = append(responses, buildFileResponse(f))
	}

	c.JSON(http.StatusOK, responses)
}

func CreateQuickEdit(c *gin.Context) {
	var req struct {
		Name     string ``
		FilePath string ``
		Remark   string ``
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

	var existing models.QuickEditFile
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

	file := models.QuickEditFile{
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

	snapshot, err := takeSnapshot(file.ID, string(data))
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
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	var file models.QuickEditFile
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
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	file, ok := findFileByID(c, id)
	if !ok {
		return
	}

	if isEnvVirtualFile(file.FilePath) {
		snap, readErr := tools.ReadAllEnvFromSystem()
		tomlStr, tomlErr := tools.MarshalEnvToTOML(snap)
		if tomlErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化 TOML 失败: " + tomlErr.Error()})
			return
		}
		warnings := []string{}
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("读取部分失败: %v", readErr))
		}
		c.JSON(http.StatusOK, gin.H{
			"content":   tomlStr,
			"filePath":  file.FilePath,
			"sizeBytes": len([]byte(tomlStr)),
			"warnings":  warnings,
		})
		return
	}

	data, err := os.ReadFile(file.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":   string(data),
		"filePath":  file.FilePath,
		"sizeBytes": len(data),
	})
}

func UpdateQuickEditContent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	file, ok := findFileByID(c, id)
	if !ok {
		return
	}

	var req struct {
		Content string ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len([]byte(req.Content)) > maxContentLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容过大，暂不支持"})
		return
	}

	if isEnvVirtualFile(file.FilePath) {
		parsed, parseErr := tools.ParseEnvFromTOML(req.Content)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "TOML 解析失败: " + parseErr.Error()})
			return
		}
		writeWarnings, writeErr := tools.WriteAllEnvToSystem(parsed)
		if writeErr != nil {
			writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnapshot, snapErr := takeSnapshot(file.ID, req.Content)
		if snapErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":    "环境变量已更新，但快照失败: " + snapErr.Error(),
				"warnings": writeWarnings,
			})
			return
		}
		file.UpdatedAt = time.Now()
		config.GetDB().Model(&models.QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
		resp := buildFileResponse(*file)
		c.JSON(http.StatusOK, gin.H{
			"message":      "环境变量已更新",
			"data":         resp,
			"snapshotId":   afterSnapshot.ID,
			"snapshotTime": afterSnapshot.CreatedAt,
			"warnings":     writeWarnings,
		})
		return
	}

	if err := os.WriteFile(file.FilePath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败 (可能需要管理员权限): " + err.Error()})
		return
	}

	afterSnapshot, snapErr := takeSnapshot(file.ID, req.Content)
	if snapErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件已更新，但快照失败: " + snapErr.Error()})
		return
	}

	file.UpdatedAt = time.Now()
	config.GetDB().Model(&models.QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)

	resp := buildFileResponse(*file)
	c.JSON(http.StatusOK, gin.H{
		"message":      "更新成功",
		"data":         resp,
		"snapshotId":   afterSnapshot.ID,
		"snapshotTime": afterSnapshot.CreatedAt,
	})
}

func ListQuickEditSnapshots(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
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
	config.GetDB().Model(&models.QuickEditSnapshot{}).Where("file_id = ?", id).Count(&total)

	var snaps []models.QuickEditSnapshot
	if err := config.GetDB().
		Where("file_id = ?", id).
		Order("created_at DESC, id DESC").
		Limit(size).
		Offset((page - 1) * size).
		Find(&snaps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	items := make([]models.QuickEditSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		items = append(items, models.QuickEditSnapshotResponse{
			ID:        s.ID,
			FileID:    s.FileID,
			SizeBytes: s.SizeBytes,
			CreatedAt: s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func GetQuickEditSnapshot(c *gin.Context) {
	idStr := c.Param("id")
	fileID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 file id"})
		return
	}

	snapIDStr := c.Param("snapshotId")
	snapID, err := strconv.Atoi(snapIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 snapshot id"})
		return
	}

	if _, ok := findFileByID(c, fileID); !ok {
		return
	}

	var snap models.QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        snap.ID,
		"fileId":    snap.FileID,
		"content":   snap.Content,
		"sizeBytes": snap.SizeBytes,
		"createdAt": snap.CreatedAt,
	})
}

func RestoreQuickEdit(c *gin.Context) {
	idStr := c.Param("id")
	fileID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	file, ok := findFileByID(c, fileID)
	if !ok {
		return
	}

	var req struct {
		SnapshotID int ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var snap models.QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", req.SnapshotID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}

	if isEnvVirtualFile(file.FilePath) {
		parsed, parseErr := tools.ParseEnvFromTOML(snap.Content)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "快照 TOML 解析失败: " + parseErr.Error()})
			return
		}
		writeWarnings, writeErr := tools.WriteAllEnvToSystem(parsed)
		if writeErr != nil {
			writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnap, snapErr := takeSnapshot(fileID, snap.Content)
		if snapErr != nil {
			writeWarnings = append(writeWarnings, "新快照失败: "+snapErr.Error())
		}
		file.UpdatedAt = time.Now()
		config.GetDB().Model(&models.QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)
		resp := buildFileResponse(*file)
		c.JSON(http.StatusOK, gin.H{
			"message":        "环境变量已回滚",
			"data":           resp,
			"fromSnapshotId": snap.ID,
			"snapshotId":     afterSnap.ID,
			"warnings":       writeWarnings,
		})
		return
	}

	if err := os.WriteFile(file.FilePath, []byte(snap.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败 (可能需要管理员权限): " + err.Error()})
		return
	}

	afterSnap, snapErr := takeSnapshot(fileID, snap.Content)
	if snapErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件已回滚，但快照失败: " + snapErr.Error()})
		return
	}

	file.UpdatedAt = time.Now()
	config.GetDB().Model(&models.QuickEditFile{}).Where("id = ?", file.ID).Update("updated_at", file.UpdatedAt)

	resp := buildFileResponse(*file)
	c.JSON(http.StatusOK, gin.H{
		"message":        "回滚成功",
		"data":           resp,
		"fromSnapshotId": snap.ID,
		"snapshotId":     afterSnap.ID,
	})
}
