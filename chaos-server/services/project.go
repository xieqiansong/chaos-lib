package services

import (
	"chaos-lib/config"
	"chaos-lib/models"
	"chaos-lib/tools"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func getProjectID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return 0, false
	}
	return id, true
}

// resolveProjectPaths 根据项目组的绝对路径与（可选的）绝对/相对路径，计算并校验最终路径。
// 优先使用 absolutePath；否则使用 relativePath 结合组根目录拼接。
// 返回绝对路径与相对路径。
func resolveProjectPaths(group models.ProjectGroup, absolutePath, relativePath string) (string, string, error) {
	var abs, rel string
	var err error

	switch {
	case absolutePath != "":
		abs = filepath.Clean(absolutePath)
		rel, err = filepath.Rel(group.AbsolutePath, abs)
		if err != nil {
			return "", "", fmt.Errorf("计算相对路径失败: %v", err)
		}
	case relativePath != "":
		rel = filepath.Clean(relativePath)
		abs = filepath.Join(group.AbsolutePath, rel)
	default:
		return "", "", fmt.Errorf("必须提供 absolutePath 或 relativePath 之一")
	}
	return abs, rel, nil
}

// CreateProject 创建项目
// 需指定所属项目组；通过 absolutePath 或 relativePath 定位，二者之一必填。
// 后端校验目录存在，并自动计算 relativePath / absolutePath，保证二者与组根目录一致。
func CreateProject(c *gin.Context) {
	var req struct {
		GroupID      int     ``
		Name         string  ``
		AbsolutePath string  ``
		RelativePath string  ``
		GitURL       *string ``
		Remark       *string ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.GroupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所属项目组ID不能为空"})
		return
	}

	var group models.ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", req.GroupID, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属项目组不存在"})
		return
	}

	abs, rel, err := resolveProjectPaths(group, req.AbsolutePath, req.RelativePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !tools.DirExists(abs) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目目录不存在: " + abs})
		return
	}

	name := req.Name
	if name == "" {
		name = filepath.Base(abs)
	}

	// 添加时间取文件夹的创建时间（Windows 下为文件创建时间），获取不到则回退为当前时间；
	// 上次访问时间默认等于添加时间。
	createdAt := tools.DirCreatedAt(abs)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	lastAccessed := createdAt

	project := models.Project{
		GroupID:        group.ID,
		Name:           name,
		AbsolutePath:   abs,
		RelativePath:   rel,
		GitURL:         req.GitURL,
		Remark:         req.Remark,
		CreatedAt:      createdAt,
		LastAccessedAt: &lastAccessed,
	}
	if err := config.GetDB().Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, project)
}

// ListProjects 查询项目列表
// 支持 ?groupId= 过滤；按 last_accessed_at 倒序、created_at 倒序排序。
// 当指定 groupId 时，额外扫描该组根目录下的「一层子目录」，将尚未入库的目录作为
// claimed=false 的未认领项合并返回，便于前端一键认领。
func ListProjects(c *gin.Context) {
	db := config.GetDB().Model(&models.Project{}).Where("is_deleted = ?", false)

	var groupID *int
	if gid := c.Query("groupId"); gid != "" {
		id, err := strconv.Atoi(gid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的groupId"})
			return
		}
		groupID = &id
		db = db.Where("group_id = ?", id)
	}

	var projects []models.Project
	if err := db.Order("last_accessed_at DESC NULLS LAST, created_at DESC, id DESC").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	// 指定项目组时：合并磁盘扫描出的未认领子目录
	if groupID != nil {
		var group models.ProjectGroup
		if err := config.GetDB().Where("id = ? AND is_deleted = ?", *groupID, false).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
			return
		}
		items := buildProjectList(group, projects)
		c.JSON(http.StatusOK, items)
		return
	}

	c.JSON(http.StatusOK, projects)
}

// buildProjectList 合并数据库项目与项目组根目录下尚未入库的子目录。
// 已入库子目录标记 claimed=true；仅存在于磁盘、未入库的标记 claimed=false。
func buildProjectList(group models.ProjectGroup, dbProjects []models.Project) []models.ProjectListItem {
	claimedSet := make(map[string]models.Project, len(dbProjects))
	for _, p := range dbProjects {
		claimedSet[filepath.Clean(p.AbsolutePath)] = p
	}

	var items []models.ProjectListItem
	// 已认领项目（保持数据库原有排序）
	for _, p := range dbProjects {
		items = append(items, models.ProjectListItem{Project: p, Claimed: true})
	}

	// 扫描组根目录下的一层子目录
	entries, err := os.ReadDir(group.AbsolutePath)
	if err != nil {
		// 目录不可读时，仅返回已认领项目
		return items
	}

	type unclaimed struct {
		item models.ProjectListItem
		name string
	}
	var unclaimedList []unclaimed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		abs := filepath.Clean(filepath.Join(group.AbsolutePath, e.Name()))
		if _, ok := claimedSet[abs]; ok {
			continue
		}
		rel, relErr := filepath.Rel(group.AbsolutePath, abs)
		if relErr != nil {
			rel = e.Name()
		}
		unclaimedList = append(unclaimedList, unclaimed{
			item: models.ProjectListItem{
				Project: models.Project{
					GroupID:      group.ID,
					Name:         e.Name(),
					AbsolutePath: abs,
					RelativePath: rel,
				},
				Claimed: false,
			},
			name: e.Name(),
		})
	}

	// 未认领项按名称排序，置于已认领项之后
	sort.Slice(unclaimedList, func(i, j int) bool {
		return unclaimedList[i].name < unclaimedList[j].name
	})
	for _, u := range unclaimedList {
		items = append(items, u.item)
	}
	return items
}

// GetProject 查询单个项目
func GetProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}
	var project models.Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	c.JSON(http.StatusOK, project)
}

// UpdateProject 更新项目基础字段（名称、Git 地址、备注）
// 注意：移动文件夹请使用 PATCH /projects/:id/move。
func UpdateProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}

	var req struct {
		Name   *string ``
		GitURL *string ``
		Remark *string ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var project models.Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	updated := map[string]interface{}{}
	if req.Name != nil {
		updated["name"] = *req.Name
	}
	if req.GitURL != nil {
		updated["git_url"] = *req.GitURL
	}
	if req.Remark != nil {
		updated["remark"] = *req.Remark
	}
	if len(updated) == 0 {
		c.JSON(http.StatusOK, project)
		return
	}
	updated["updated_at"] = time.Now()

	if err := config.GetDB().Model(&project).Updates(updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	config.GetDB().First(&project, id)
	c.JSON(http.StatusOK, project)
}

// MoveProject 移动项目文件夹（安全策略：先复制、后更新数据库、再回收站删除原目录）
// 请求体：{ targetGroupId, targetRelativePath } 或 { targetAbsPath }
// 流程：
//  1. 校验原目录存在、目标目录不存在
//  2. 复制原目录到目标
//  3. 事务更新 group_id / absolute_path / relative_path
//  4. 数据库更新成功后，将原目录送入回收站（非永久删除）
//
// 若第 3 步失败，仅清理复制出的副本，原目录保持不动，数据安全。
func MoveProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}

	var req struct {
		TargetGroupID      int    ``
		TargetRelativePath string ``
		TargetAbsPath      string ``
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TargetGroupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标项目组ID不能为空"})
		return
	}

	var project models.Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var group models.ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", req.TargetGroupID, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标项目组不存在"})
		return
	}

	newAbs, newRel, err := resolveProjectPaths(group, req.TargetAbsPath, req.TargetRelativePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	oldAbs := project.AbsolutePath
	moved := newAbs != oldAbs

	// 第 1~2 步：检查原目录存在、目标不存在，然后复制到目标（不删除原目录）
	if moved {
		if err := tools.MoveProjectFolder(oldAbs, newAbs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "复制文件夹失败: " + err.Error()})
			return
		}
	}

	// 第 3 步：更新数据库（事务）
	tx := config.GetDB().Begin()
	updates := map[string]interface{}{
		"group_id":      group.ID,
		"absolute_path": newAbs,
		"relative_path": newRel,
		"updated_at":    time.Now(),
	}
	if err := tx.Model(&project).Updates(updates).Error; err != nil {
		tx.Rollback()
		// 元数据更新失败：清理已拷贝的目标副本，保留原目录（安全）
		if moved {
			_ = tools.RemoveDirSafe(newAbs)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新路径失败: " + err.Error()})
		return
	}
	tx.Commit()

	// 第 4 步：数据库更新成功后，将原目录送入回收站（而非永久删除）
	var recycleErr error
	if moved {
		recycleErr = tools.MoveToRecycleBin(oldAbs)
	}

	config.GetDB().First(&project, id)
	resp := gin.H{
		"message":    "移动成功",
		"moved":      moved,
		"project":    project,
		"oldAbsPath": oldAbs,
		"newAbsPath": newAbs,
	}
	if recycleErr != nil {
		resp["recycleWarning"] = "原目录未能送入回收站，请手动清理: " + recycleErr.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// AccessProject 记录项目访问（更新 last_accessed_at）
// 前端在打开/浏览项目时调用，用于排序与统计上次访问时间。
func AccessProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}

	var project models.Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	now := time.Now()
	if err := config.GetDB().Model(&project).Updates(map[string]interface{}{
		"last_accessed_at": now,
		"updated_at":       now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新访问时间失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已记录访问", "lastAccessedAt": now})
}

// DeleteProject 删除项目（软删除）
// 默认仅删除元数据；?removeDir=true 时同时物理删除磁盘目录。
func DeleteProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}
	removeDir := c.Query("removeDir") == "true"

	var project models.Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	if err := config.GetDB().Model(&project).Updates(map[string]interface{}{
		"is_deleted": true,
		"updated_at": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	if removeDir {
		if err := tools.RemoveDirSafe(project.AbsolutePath); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message":        "已软删除项目，但物理目录删除失败",
				"dirRemoveError": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "removeDir": removeDir})
}
