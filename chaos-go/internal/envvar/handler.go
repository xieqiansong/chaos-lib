package envvar

import (
	"chaos-go/config"
	"chaos-go/internal/quickedit"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ensureEnvFileID 确保环境变量虚拟文件存在于 quickedit 表中
func ensureEnvFileID() (int, error) {
	var file quickedit.QuickEditFile
	err := config.GetDB().Where("file_path = ?", quickedit.EnvVirtualFilePath).First(&file).Error
	if err == nil {
		return file.ID, nil
	}
	snap, _ := ReadAllEnvFromSystem()
	tomlStr, tomlErr := MarshalEnvToTOML(snap)
	if tomlErr != nil {
		return 0, tomlErr
	}
	file = quickedit.QuickEditFile{
		Name:      quickedit.EnvVirtualFileName,
		FilePath:  quickedit.EnvVirtualFilePath,
		Remark:    quickedit.EnvVirtualRemark,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := config.GetDB().Create(&file).Error; err != nil {
		return 0, fmt.Errorf("创建虚拟文件失败: %w", err)
	}
	_, snapErr := quickedit.TakeSnapshot(file.ID, tomlStr)
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
	snap, readErr := ReadAllEnvFromSystem()
	warnings := []string{}
	if readErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取环境变量部分失败: %v", readErr))
	}
	var latest quickedit.QuickEditSnapshot
	config.GetDB().Where("file_id = ?", fileID).Order("created_at DESC, id DESC").First(&latest)
	resp := EnvGetResponse{
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
	snap, readErr := ReadAllEnvFromSystem()
	tomlStr, tomlErr := MarshalEnvToTOML(snap)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}
	afterSnap, snapErr := quickedit.TakeSnapshot(fileID, tomlStr)
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
	var req EnvPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.System == nil && req.User == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不能为空（至少需要 system 或 user 之一）"})
		return
	}
	snap, readErr := ReadAllEnvFromSystem()
	if snap == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取环境变量失败: %v", readErr)})
		return
	}
	ApplySectionPatch(&snap.System, req.System)
	ApplySectionPatch(&snap.User, req.User)
	writeWarnings, writeErr := WriteAllEnvToSystem(snap)
	if writeErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
	}
	tomlStr, tomlErr := MarshalEnvToTOML(snap)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}
	afterSnap, snapErr := quickedit.TakeSnapshot(fileID, tomlStr)
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
	var req EnvPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	current, _ := ReadAllEnvFromSystem()
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
	writeWarnings, writeErr := WriteAllEnvToSystem(current)
	if writeErr != nil {
		writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
	}
	tomlStr, tomlErr := MarshalEnvToTOML(current)
	if tomlErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tomlErr.Error()})
		return
	}
	afterSnap, snapErr := quickedit.TakeSnapshot(fileID, tomlStr)
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
	var snap quickedit.QuickEditSnapshot
	if err := config.GetDB().Where("id = ? AND file_id = ?", snapID, fileID).First(&snap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
		return
	}
	parsed, parseErr := ParseEnvFromTOML(snap.Content)
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

// ReadVirtualContent 供 quickedit 回调使用：读系统环境变量并序列化为 TOML
func ReadVirtualContent() (content string, sizeBytes int, err error) {
	snap, err := ReadAllEnvFromSystem()
	if err != nil {
		return "", 0, err
	}
	content, tomlErr := MarshalEnvToTOML(snap)
	if tomlErr != nil {
		return "", 0, tomlErr
	}
	return content, len([]byte(content)), nil
}

// WriteVirtualContent 供 quickedit 回调使用：解析 TOML 并写入系统环境变量
func WriteVirtualContent(content string) (warnings []string, err error) {
	snap, err := ParseEnvFromTOML(content)
	if err != nil {
		return nil, err
	}
	return WriteAllEnvToSystem(snap)
}
