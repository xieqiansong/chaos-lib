package services

import (
	"chaos-lib/config"
	"chaos-lib/models"
	"chaos-lib/tools"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func getGroupID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目组ID"})
		return 0, false
	}
	return id, true
}

// getRecycleBinGroup 返回唯一的回收站项目组。
// 若不存在则直接返回错误响应，调用方应据此提前返回。
func getRecycleBinGroup(c *gin.Context) (models.ProjectGroup, bool) {
	var group models.ProjectGroup
	if err := config.GetDB().Where("is_recycle_bin = ? AND is_deleted = ?", true, false).First(&group).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "未配置回收站项目组，请先创建一个 is_recycle_bin=true 的项目组",
			"suggestion": "POST /api/projectGroups 并携带 {\"isRecycleBin\": true, \"name\": \"回收站\", \"absolutePath\": \"<回收站目录>\"}",
		})
		return group, false
	}
	return group, true
}

// CreateProjectGroup 创建项目组
// 校验根目录是否存在；不存在则尝试创建。
// 若 isRecycleBin=true，则强制唯一（已存在回收站分组时拒绝），名称缺省为「回收站」。
func CreateProjectGroup(c *gin.Context) {
	var req struct {
		Name         string  ``
		OrderNum     *int    ``
		AbsolutePath string  ``
		Remark       *string ``
		IsRecycleBin *bool   ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isRecycle := req.IsRecycleBin != nil && *req.IsRecycleBin
	if isRecycle {
		// 回收站分组唯一
		var existing models.ProjectGroup
		if err := config.GetDB().Where("is_recycle_bin = ? AND is_deleted = ?", true, false).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "回收站项目组已存在，不能重复创建", "existingId": existing.ID})
			return
		}
		if req.Name == "" {
			req.Name = "回收站"
		}
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目组名称不能为空"})
		return
	}
	if req.AbsolutePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "根目录绝对路径不能为空"})
		return
	}

	orderNum := 0
	if req.OrderNum != nil {
		orderNum = *req.OrderNum
	}

	// 根目录不存在则创建（允许用户直接以绝对路径登记一个尚未创建的目录）
	if !tools.DirExists(req.AbsolutePath) {
		if err := tools.MkdirAllSafe(req.AbsolutePath); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "根目录不存在且创建失败: " + err.Error()})
			return
		}
	}

	group := models.ProjectGroup{
		Name:         req.Name,
		OrderNum:     orderNum,
		AbsolutePath: filepath.Clean(req.AbsolutePath),
		Remark:       req.Remark,
		IsRecycleBin: isRecycle,
	}
	if err := config.GetDB().Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目组失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

// ListProjectGroups 查询项目组列表
// 按 order_num 升序、created_at 倒序、id 倒序排序。
func ListProjectGroups(c *gin.Context) {
	var groups []models.ProjectGroup
	if err := config.GetDB().Where("is_deleted = ?", false).
		Order("order_num ASC, created_at DESC, id DESC").
		Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// GetProjectGroup 查询单个项目组
func GetProjectGroup(c *gin.Context) {
	id, ok := getGroupID(c)
	if !ok {
		return
	}

	var group models.ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}
	c.JSON(http.StatusOK, group)
}

// UpdateProjectGroup 更新项目组（名称、根目录、备注）
// 当根目录发生变化时，级联重算其下所有项目的 AbsolutePath（仅元数据，不动磁盘）。
func UpdateProjectGroup(c *gin.Context) {
	id, ok := getGroupID(c)
	if !ok {
		return
	}

	var req struct {
		Name         *string ``
		OrderNum     *int    ``
		AbsolutePath *string ``
		Remark       *string ``
		IsRecycleBin *bool   ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var group models.ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}

	// 回收站分组不可被改造为非回收站，也不可把普通分组改造成回收站（若已存在其它回收站）
	if req.IsRecycleBin != nil {
		wantRecycle := *req.IsRecycleBin
		if wantRecycle != group.IsRecycleBin {
			if wantRecycle {
				// 要避免出现第二个回收站
				var other models.ProjectGroup
				if err := config.GetDB().Where("is_recycle_bin = ? AND is_deleted = ? AND id <> ?", true, false, id).First(&other).Error; err == nil {
					c.JSON(http.StatusConflict, gin.H{"error": "回收站项目组已存在，不能重复创建", "existingId": other.ID})
					return
				}
			}
		}
	}

	updated := map[string]interface{}{}
	if req.Name != nil {
		updated["name"] = *req.Name
	}
	if req.OrderNum != nil {
		updated["order_num"] = *req.OrderNum
	}
	if req.Remark != nil {
		updated["remark"] = *req.Remark
	}
	if req.IsRecycleBin != nil {
		updated["is_recycle_bin"] = *req.IsRecycleBin
	}

	// 根目录变更：仅更新元数据并级联重算子项目绝对路径
	if req.AbsolutePath != nil && *req.AbsolutePath != group.AbsolutePath {
		newRoot := filepath.Clean(*req.AbsolutePath)
		updated["absolute_path"] = newRoot

		tx := config.GetDB().Begin()
		if err := tx.Model(&group).Updates(updated).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}

		// 重算所有子项目绝对路径 = 新根目录 + 相对路径
		var projects []models.Project
		if err := tx.Where("group_id = ? AND is_deleted = ?", id, false).Find(&projects).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询子项目失败: " + err.Error()})
			return
		}
		for _, p := range projects {
			newAbs := filepath.Join(newRoot, p.RelativePath)
			if err := tx.Model(&p).Updates(map[string]interface{}{
				"absolute_path": newAbs,
				"updated_at":    time.Now(),
			}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "重算子项目路径失败: " + err.Error()})
				return
			}
		}
		tx.Commit()

		// 重新读取最新记录返回
		config.GetDB().First(&group, id)
		c.JSON(http.StatusOK, group)
		return
	}

	if len(updated) == 0 {
		c.JSON(http.StatusOK, group)
		return
	}
	updated["updated_at"] = time.Now()
	if err := config.GetDB().Model(&group).Updates(updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	config.GetDB().First(&group, id)
	c.JSON(http.StatusOK, group)
}

// DeleteProjectGroup 删除项目组（软删除）
// 默认在有子项目时拒绝删除；?cascade=true 时级联软删其下所有项目。
func DeleteProjectGroup(c *gin.Context) {
	id, ok := getGroupID(c)
	if !ok {
		return
	}
	cascade := c.Query("cascade") == "true"

	var group models.ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}

	if group.IsRecycleBin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回收站项目组不可删除"})
		return
	}

	var childCount int64
	if err := config.GetDB().Model(&models.Project{}).
		Where("group_id = ? AND is_deleted = ?", id, false).
		Count(&childCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询子项目失败: " + err.Error()})
		return
	}

	if !cascade && childCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "该项目组下还有项目，无法删除",
			"childCount": childCount,
			"suggestion": "请先移除组内项目，或使用 ?cascade=true 级联删除",
		})
		return
	}

	tx := config.GetDB().Begin()
	if cascade && childCount > 0 {
		if err := tx.Model(&models.Project{}).
			Where("group_id = ? AND is_deleted = ?", id, false).
			Updates(map[string]interface{}{"is_deleted": true, "updated_at": time.Now()}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "级联删除项目失败: " + err.Error()})
			return
		}
	}
	if err := tx.Model(&group).Updates(map[string]interface{}{"is_deleted": true, "updated_at": time.Now()}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	tx.Commit()

	resp := gin.H{"message": "删除成功", "cascade": cascade}
	if cascade {
		resp["deletedProjectCount"] = childCount
	}
	c.JSON(http.StatusOK, resp)
}
