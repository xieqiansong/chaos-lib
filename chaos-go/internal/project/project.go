package project

import (
	"chaos-go/config"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

type ProjectGroup struct {
	ID           int       `gorm:"primaryKey"`
	Name         string    ``
	OrderNum     int       `gorm:"default:0"`
	AbsolutePath string    ``
	Remark       *string   ``
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted    bool      `gorm:"default:false"`
	IsRecycleBin bool      `gorm:"default:false"`
}

func (ProjectGroup) TableName() string { return "project_groups" }

type Project struct {
	ID             int        `gorm:"primaryKey"`
	GroupID        int        ``
	Name           string     ``
	AbsolutePath   string     ``
	RelativePath   string     ``
	GitURL         *string    ``
	Remark         *string    ``
	LastAccessedAt *time.Time ``
	CreatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	IsDeleted      bool       `gorm:"default:false"`
}

func (Project) TableName() string { return "projects" }

type ProjectListItem struct {
	Project
	Claimed bool ``
}

// ── 辅助函数 ──────────────────────────────────────────────────────

func getProjectID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
		return 0, false
	}
	return id, true
}

func getGroupID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目组ID"})
		return 0, false
	}
	return id, true
}

func getRecycleBinGroup(c *gin.Context) (ProjectGroup, bool) {
	var group ProjectGroup
	if err := config.GetDB().Where("is_recycle_bin = ? AND is_deleted = ?", true, false).First(&group).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "未配置回收站项目组，请先创建一个 is_recycle_bin=true 的项目组",
			"suggestion": "POST /api/projectGroups 并携带 {\"isRecycleBin\": true, \"name\": \"回收站\", \"absolutePath\": \"<回收站目录>\"}",
		})
		return group, false
	}
	return group, true
}

func resolveProjectPaths(group ProjectGroup, absolutePath, relativePath string) (string, string, error) {
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

func resolveRecycleTarget(recycleGroup ProjectGroup, project Project) (string, string, error) {
	if err := os.MkdirAll(recycleGroup.AbsolutePath, 0o755); err != nil {
		return "", "", fmt.Errorf("回收站根目录不存在且创建失败: %v", err)
	}
	base := filepath.Join(recycleGroup.AbsolutePath, project.Name)
	candidate := base
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); err != nil {
			break
		}
		candidate = fmt.Sprintf("%s_%d", base, i)
	}
	abs := filepath.Clean(candidate)
	rel, err := filepath.Rel(recycleGroup.AbsolutePath, abs)
	if err != nil {
		return "", "", fmt.Errorf("计算回收站相对路径失败: %v", err)
	}
	return abs, rel, nil
}

// ── Project Handlers ───────────────────────────────────────────────

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

	var group ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", req.GroupID, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "所属项目组不存在"})
		return
	}

	abs, rel, err := resolveProjectPaths(group, req.AbsolutePath, req.RelativePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目目录不存在: " + abs})
		return
	}

	name := req.Name
	if name == "" {
		name = filepath.Base(abs)
	}

	var existing Project
	if err := config.GetDB().Unscoped().Where("absolute_path = ? AND is_deleted = ?", abs, true).First(&existing).Error; err == nil {
		updates := map[string]interface{}{
			"is_deleted": false, "name": name, "group_id": group.ID,
			"relative_path": rel, "git_url": req.GitURL, "remark": req.Remark,
			"updated_at": time.Now(),
		}
		if uerr := config.GetDB().Model(&existing).Updates(updates).Error; uerr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "还原项目失败: " + uerr.Error()})
			return
		}
		config.GetDB().First(&existing, existing.ID)
		c.JSON(http.StatusOK, gin.H{"message": "项目已还原（reactivated）", "project": existing})
		return
	}

	createdAt := DirCreatedAt(abs)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	lastAccessed := createdAt

	project := Project{
		GroupID: group.ID, Name: name, AbsolutePath: abs, RelativePath: rel,
		GitURL: req.GitURL, Remark: req.Remark,
		CreatedAt: createdAt, LastAccessedAt: &lastAccessed,
	}
	if err := config.GetDB().Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, project)
}

func ListProjects(c *gin.Context) {
	db := config.GetDB().Model(&Project{}).Where("is_deleted = ?", false)
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

	if groupID == nil {
		var recycleGroup ProjectGroup
		if err := config.GetDB().Where("is_recycle_bin = ? AND is_deleted = ?", true, false).First(&recycleGroup).Error; err == nil {
			db = db.Where("group_id <> ?", recycleGroup.ID)
		}
	}

	var projects []Project
	if err := db.Order("last_accessed_at DESC NULLS LAST, created_at DESC, id DESC").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	if groupID != nil {
		var group ProjectGroup
		if err := config.GetDB().Where("id = ? AND is_deleted = ?", *groupID, false).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
			return
		}
		if group.IsRecycleBin {
			c.JSON(http.StatusOK, projects)
			return
		}
		items := buildProjectList(group, projects)
		c.JSON(http.StatusOK, items)
		return
	}
	c.JSON(http.StatusOK, projects)
}

func buildProjectList(group ProjectGroup, dbProjects []Project) []ProjectListItem {
	claimedSet := make(map[string]Project, len(dbProjects))
	for _, p := range dbProjects {
		claimedSet[filepath.Clean(p.AbsolutePath)] = p
	}

	var items []ProjectListItem
	for _, p := range dbProjects {
		items = append(items, ProjectListItem{Project: p, Claimed: true})
	}

	entries, err := os.ReadDir(group.AbsolutePath)
	if err != nil {
		return items
	}

	type unclaimed struct {
		item ProjectListItem
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
			item: ProjectListItem{
				Project: Project{GroupID: group.ID, Name: e.Name(), AbsolutePath: abs, RelativePath: rel},
				Claimed: false,
			},
			name: e.Name(),
		})
	}

	sort.Slice(unclaimedList, func(i, j int) bool {
		return unclaimedList[i].name < unclaimedList[j].name
	})
	for _, u := range unclaimedList {
		items = append(items, u.item)
	}
	return items
}

func GetProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}
	var project Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	c.JSON(http.StatusOK, project)
}

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

	var project Project
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

	var project Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var group ProjectGroup
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

	if moved {
		if err := MoveProjectFolder(oldAbs, newAbs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "复制文件夹失败: " + err.Error()})
			return
		}
	}

	tx := config.GetDB().Begin()
	updates := map[string]interface{}{
		"group_id": group.ID, "absolute_path": newAbs,
		"relative_path": newRel, "updated_at": time.Now(),
	}
	if err := tx.Model(&project).Updates(updates).Error; err != nil {
		tx.Rollback()
		if moved {
			_ = RemoveDirSafe(newAbs)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新路径失败: " + err.Error()})
		return
	}
	tx.Commit()

	var recycleErr error
	if moved {
		recycleErr = MoveToRecycleBin(oldAbs)
	}

	config.GetDB().First(&project, id)
	resp := gin.H{
		"message": "移动成功", "moved": moved, "project": project,
		"oldAbsPath": oldAbs, "newAbsPath": newAbs,
	}
	if recycleErr != nil {
		resp["recycleWarning"] = "原目录未能送入回收站，请手动清理: " + recycleErr.Error()
	}
	c.JSON(http.StatusOK, resp)
}

func AccessProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}
	var project Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	now := time.Now()
	if err := config.GetDB().Model(&project).Updates(map[string]interface{}{
		"last_accessed_at": now, "updated_at": now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新访问时间失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已记录访问", "lastAccessedAt": now})
}

func DeleteProject(c *gin.Context) {
	id, ok := getProjectID(c)
	if !ok {
		return
	}
	var project Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	recycleGroup, isRecycle := getRecycleBinGroup(c)
	if !isRecycle {
		return
	}

	if project.GroupID == recycleGroup.ID {
		if err := RemoveDirSafe(project.AbsolutePath); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message": "已软删除项目，但物理目录删除失败", "dirRemoveError": err.Error(),
			})
			return
		}
		if err := config.GetDB().Unscoped().Delete(&project).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除记录失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已永久删除"})
		return
	}

	newAbs, newRel, err := resolveRecycleTarget(recycleGroup, project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := MoveProjectFolder(project.AbsolutePath, newAbs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移入回收站失败: " + err.Error()})
		return
	}

	tx := config.GetDB().Begin()
	if err := tx.Model(&project).Updates(map[string]interface{}{
		"group_id": recycleGroup.ID, "absolute_path": newAbs,
		"relative_path": newRel, "is_deleted": false, "updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		_ = RemoveDirSafe(newAbs)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新路径失败: " + err.Error()})
		return
	}
	tx.Commit()

	oldAbs := project.AbsolutePath
	if err := RemoveDirSafe(oldAbs); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "已移入回收站，但原目录未能删除", "recycleWarning": err.Error(), "newAbsPath": newAbs,
		})
		return
	}

	config.GetDB().First(&project, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "已移入回收站", "recycled": true, "project": project, "newAbsPath": newAbs,
	})
}

func RestoreProject(c *gin.Context) {
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

	var project Project
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	var targetGroup ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", req.TargetGroupID, false).First(&targetGroup).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标项目组不存在"})
		return
	}

	newAbs, newRel, err := resolveProjectPaths(targetGroup, req.TargetAbsPath, req.TargetRelativePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := MoveProjectFolder(project.AbsolutePath, newAbs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "复制到目标分组失败: " + err.Error()})
		return
	}

	tx := config.GetDB().Begin()
	if err := tx.Model(&project).Updates(map[string]interface{}{
		"group_id": targetGroup.ID, "absolute_path": newAbs,
		"relative_path": newRel, "updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		_ = RemoveDirSafe(newAbs)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新路径失败: " + err.Error()})
		return
	}
	tx.Commit()

	oldAbs := project.AbsolutePath
	if err := RemoveDirSafe(oldAbs); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "已还原，但回收站内原目录未能删除", "recycleWarning": err.Error(), "newAbsPath": newAbs,
		})
		return
	}

	config.GetDB().First(&project, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "已还原", "project": project, "newAbsPath": newAbs,
	})
}

// ── ProjectGroup Handlers ──────────────────────────────────────────

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
		var existing ProjectGroup
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

	if err := os.MkdirAll(req.AbsolutePath, 0o755); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "根目录不存在且创建失败: " + err.Error()})
		return
	}

	group := ProjectGroup{
		Name: req.Name, OrderNum: orderNum,
		AbsolutePath: filepath.Clean(req.AbsolutePath),
		Remark:       req.Remark, IsRecycleBin: isRecycle,
	}
	if err := config.GetDB().Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建项目组失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

func ListProjectGroups(c *gin.Context) {
	var groups []ProjectGroup
	if err := config.GetDB().Where("is_deleted = ?", false).
		Order("order_num ASC, created_at DESC, id DESC").
		Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func GetProjectGroup(c *gin.Context) {
	id, ok := getGroupID(c)
	if !ok {
		return
	}
	var group ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}
	c.JSON(http.StatusOK, group)
}

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

	var group ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}

	if req.IsRecycleBin != nil {
		wantRecycle := *req.IsRecycleBin
		if wantRecycle != group.IsRecycleBin {
			if wantRecycle {
				var other ProjectGroup
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

	if req.AbsolutePath != nil && *req.AbsolutePath != group.AbsolutePath {
		newRoot := filepath.Clean(*req.AbsolutePath)
		updated["absolute_path"] = newRoot

		tx := config.GetDB().Begin()
		if err := tx.Model(&group).Updates(updated).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}

		var projects []Project
		if err := tx.Where("group_id = ? AND is_deleted = ?", id, false).Find(&projects).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询子项目失败: " + err.Error()})
			return
		}
		for _, p := range projects {
			newAbs := filepath.Join(newRoot, p.RelativePath)
			if err := tx.Model(&p).Updates(map[string]interface{}{
				"absolute_path": newAbs, "updated_at": time.Now(),
			}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "重算子项目路径失败: " + err.Error()})
				return
			}
		}
		tx.Commit()

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

func DeleteProjectGroup(c *gin.Context) {
	id, ok := getGroupID(c)
	if !ok {
		return
	}
	cascade := c.Query("cascade") == "true"

	var group ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
		return
	}

	if group.IsRecycleBin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "回收站项目组不可删除"})
		return
	}

	var childCount int64
	if err := config.GetDB().Model(&Project{}).Where("group_id = ? AND is_deleted = ?", id, false).Count(&childCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询子项目失败: " + err.Error()})
		return
	}

	if !cascade && childCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "该项目组下还有项目，无法删除", "childCount": childCount,
			"suggestion": "请先移除组内项目，或使用 ?cascade=true 级联删除",
		})
		return
	}

	tx := config.GetDB().Begin()
	if cascade && childCount > 0 {
		if err := tx.Model(&Project{}).Where("group_id = ? AND is_deleted = ?", id, false).
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
