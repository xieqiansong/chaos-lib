package services

import (
	"chaos-go/config"
	"chaos-go/models"
	"chaos-go/tools"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func ensureEnvFileID() (int, error) {
	var file models.QuickEditFile
	err := config.GetDB().Where("file_path = ?", models.EnvVirtualFilePath).First(&file).Error
	if err == nil {
		return file.ID, nil
	}

	snap, _ := tools.ReadAllEnvFromSystem()
	tomlStr, tomlErr := tools.MarshalEnvToTOML(snap)
	if tomlErr != nil {
		return 0, tomlErr
	}

	file = models.QuickEditFile{
		Name:      models.EnvVirtualFileName,
		FilePath:  models.EnvVirtualFilePath,
		Remark:    models.EnvVirtualRemark,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := config.GetDB().Create(&file).Error; err != nil {
		return 0, fmt.Errorf("创建虚拟文件失败: %w", err)
	}

	_, snapErr := takeSnapshot(file.ID, tomlStr)
	if snapErr != nil {
		return 0, snapErr
	}
	return file.ID, nil
}

func GetEnvVariables(c *gin.Context) {
	fileID, err := ensureEnvFileID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	snap, readErr := tools.ReadAllEnvFromSystem()
	warnings := []string{}
	if readErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取环境变量部分失败: %v", readErr))
	}

	var latest models.QuickEditSnapshot
	config.GetDB().Where("file_id = ?", fileID).Order("created_at DESC, id DESC").First(&latest)

	resp := models.EnvGetResponse{
		Meta:         snap.Meta,
		System:       snap.System,
		User:         snap.User,
		SnapshotID:   latest.ID,
		SnapshotTime: "",
		Warnings:     warnings,
	}
	if latest.ID > 0 {
		resp.SnapshotTime = latest.CreatedAt.Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, resp)
}

func SyncEnvVariables(c *gin.Context) {
	fileID, err := ensureEnvFileID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	snap, readErr := tools.ReadAllEnvFromSystem()
	tomlStr, tomlErr := tools.MarshalEnvToTOML(snap)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}

	afterSnap, snapErr := takeSnapshot(fileID, tomlStr)
	if snapErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("快照失败: %v", snapErr)})
		return
	}

	warnings := []string{}
	if readErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取部分失败: %v", readErr))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "已从系统刷新并生成新快照",
		"snapshotId":   afterSnap.ID,
		"snapshotTime": afterSnap.CreatedAt.Format(time.RFC3339),
		"warnings":     warnings,
	})
}

func PatchEnvVariables(c *gin.Context) {
	fileID, err := ensureEnvFileID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req models.EnvPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.System == nil && req.User == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不能为空（至少需要 system 或 user 之一）"})
		return
	}

	snap, readErr := tools.ReadAllEnvFromSystem()
	if snap == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取环境变量失败: %v", readErr)})
		return
	}

	tools.ApplySectionPatch(&snap.System, req.System)
	tools.ApplySectionPatch(&snap.User, req.User)

	writeWarnings, writeErr := tools.WriteAllEnvToSystem(snap)
	if writeErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
	}

	tomlStr, tomlErr := tools.MarshalEnvToTOML(snap)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}

	afterSnap, snapErr := takeSnapshot(fileID, tomlStr)
	if snapErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("快照失败: %v", snapErr))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "更新成功",
		"snapshotId":   afterSnap.ID,
		"snapshotTime": afterSnap.CreatedAt.Format(time.RFC3339),
		"warnings":     writeWarnings,
	})
}

func PutEnvVariables(c *gin.Context) {
	fileID, err := ensureEnvFileID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req models.EnvPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	current, _ := tools.ReadAllEnvFromSystem()
	if req.System != nil {
		current.System = *req.System
		if current.System == nil {
			current.System = map[string]string{}
		}
	}
	if req.User != nil {
		current.User = *req.User
		if current.User == nil {
			current.User = map[string]string{}
		}
	}

	writeWarnings, writeErr := tools.WriteAllEnvToSystem(current)
	if writeErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
	}

	tomlStr, tomlErr := tools.MarshalEnvToTOML(current)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}

	afterSnap, snapErr := takeSnapshot(fileID, tomlStr)
	if snapErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("快照失败: %v", snapErr))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "覆盖成功",
		"snapshotId":   afterSnap.ID,
		"snapshotTime": afterSnap.CreatedAt.Format(time.RFC3339),
		"warnings":     writeWarnings,
	})
}

func GetEnvSnapshotDetail(c *gin.Context) {
	fileID, err := ensureEnvFileID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	snapIDStr := c.Param("snapshotId")
	snapID, err := strconv.Atoi(snapIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 snapshot id"})
		return
	}

	var snap models.QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}

	parsed, parseErr := tools.ParseEnvFromTOML(snap.Content)
	if parseErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"id":         snap.ID,
			"fileId":     snap.FileID,
			"rawContent": snap.Content,
			"parseError": parseErr.Error(),
			"createdAt":  snap.CreatedAt.Format(time.RFC3339),
			"sizeBytes":  snap.SizeBytes,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         snap.ID,
		"fileId":     snap.FileID,
		"meta":       parsed.Meta,
		"system":     parsed.System,
		"user":       parsed.User,
		"rawContent": snap.Content,
		"createdAt":  snap.CreatedAt.Format(time.RFC3339),
		"sizeBytes":  snap.SizeBytes,
	})
}
