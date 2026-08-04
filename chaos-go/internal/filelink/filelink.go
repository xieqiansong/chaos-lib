package filelink

import (
	"chaos-go/config"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

type FileLink struct {
	ID         int    `gorm:"primaryKey"`
	SourcePath string ``
	TargetPath string ``
	Status     bool   ``
	Remark     string ``
	Sort       int    `gorm:"default:0"`
}

type FileLinkResponse struct {
	ID         int    `json:"id"`
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	Status     bool   `json:"status"`
	Remark     string `json:"remark"`
	Sort       int    `json:"sort"`
	LinkStatus string `json:"linkStatus"`
}

// ── 辅助 ──────────────────────────────────────────────────────────

func checkLinkStatus(sourcePath, targetPath string, enabled bool) string {
	_, err := os.Lstat(targetPath)
	if err != nil {
		if enabled {
			return "missing"
		}
		return "none"
	}
	if !enabled {
		return "invalid"
	}
	actualTarget, err := os.Readlink(targetPath)
	if err == nil {
		absActual, _ := filepath.Abs(actualTarget)
		absSource, _ := filepath.Abs(sourcePath)
		if strings.EqualFold(absActual, absSource) {
			return "normal"
		}
	}
	return "conflict"
}

// ── Handlers ──────────────────────────────────────────────────────

func GetFileLinks(c *gin.Context) {
	var links []FileLink
	config.GetDB().Debug().Order("sort ASC, id ASC").Find(&links)
	responses := make([]FileLinkResponse, 0, len(links))
	for _, link := range links {
		responses = append(responses, FileLinkResponse{
			ID: link.ID, SourcePath: link.SourcePath, TargetPath: link.TargetPath,
			Status: link.Status, Remark: link.Remark, Sort: link.Sort,
			LinkStatus: checkLinkStatus(link.SourcePath, link.TargetPath, link.Status),
		})
	}
	c.JSON(200, responses)
}

func CreateFileLink(c *gin.Context) {
	var req struct {
		SourcePath string `json:"sourcePath"`
		TargetPath string `json:"targetPath"`
		Remark     string `json:"remark"`
		Sort       int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.SourcePath == "" || req.TargetPath == "" {
		c.JSON(400, gin.H{"error": "源路径和目标路径不能为空"})
		return
	}
	link := FileLink{SourcePath: req.SourcePath, TargetPath: req.TargetPath, Status: false, Remark: req.Remark, Sort: req.Sort}
	result := config.GetDB().Create(&link)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": "创建失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "创建成功", "data": link})
}

func DeleteFileLink(c *gin.Context) {
	id := c.Param("id")
	var link FileLink
	result := config.GetDB().First(&link, "id = ?", id)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "文件连接不存在"})
		return
	}
	if link.Status {
		os.Remove(link.TargetPath)
	}
	result = config.GetDB().Delete(&link)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": "删除失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "删除成功"})
}

func UpdateFileLink(c *gin.Context) {
	id := c.Param("id")
	var link FileLink
	result := config.GetDB().First(&link, "id = ?", id)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "文件连接不存在"})
		return
	}
	var req struct {
		Remark string `json:"remark"`
		Sort   int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	link.Remark = req.Remark
	link.Sort = req.Sort
	result = config.GetDB().Save(&link)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": "更新失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "更新成功", "data": link})
}

func UpdateFileLinkStatus(c *gin.Context) {
	id := c.Param("id")
	var link FileLink
	result := config.GetDB().First(&link, "id = ?", id)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "文件连接不存在"})
		return
	}
	var request struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if request.Status {
		if link.Status {
			c.JSON(400, gin.H{"error": "文件连接已启用"})
			return
		}
		if _, err := os.Stat(link.SourcePath); os.IsNotExist(err) {
			c.JSON(400, gin.H{"error": "源路径不存在"})
			return
		}
		if _, err := os.Lstat(link.TargetPath); err == nil {
			c.JSON(400, gin.H{"error": "目标路径已存在，请先删除或移动"})
			return
		}
		err := CreateJunction(link.SourcePath, link.TargetPath)
		if err != nil {
			c.JSON(500, gin.H{"error": "创建目录联接点失败: " + err.Error()})
			return
		}
		link.Status = true
	} else {
		if !link.Status {
			c.JSON(400, gin.H{"error": "文件连接已禁用"})
			return
		}
		os.Remove(link.TargetPath)
		link.Status = false
	}
	result = config.GetDB().Save(&link)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": "更新状态失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "状态更新成功", "data": link})
}
