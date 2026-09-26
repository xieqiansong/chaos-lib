// Package project 是「项目管理」：按 ProjectGroup / Project 组织本地项目，
// 记录 Git URL 与访问时间，支持移动（同卷 rename / 跨卷 copy）、访问、删除。
//
// 本包遵循业务模块脚手架基线：模型嵌入 crud.BaseModel，标准 CRUD 交给通用 crud，
// 磁盘副作用（建组时建根目录、删组级联删项目、删项目清目录、移动时搬目录）经
// crud.Opts 的 Before/After 回调注入；项目列表需合并「磁盘未认领目录」，属基线之外的
// 派生行为，由 ListHandler 自定义挂载。所有路由经 Register 自包含挂载，routes.go 只编排调用。
package project

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"chaos-go/config"
	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

// ProjectGroup 项目组：拥有一个根目录（AbsolutePath），其下项目通过 RelativePath 相对该根目录定位。
type ProjectGroup struct {
	crud.BaseModel
	Name         string  `json:"Name"`
	OrderNum     int     `json:"OrderNum" gorm:"default:0"`
	AbsolutePath string  `json:"AbsolutePath"`
	Remark       *string `json:"Remark"`
}

func (ProjectGroup) TableName() string { return "project_groups" }

// Project 项目：绝对路径 = 所属项目组绝对路径 + 相对路径。
type Project struct {
	crud.BaseModel
	GroupID        int        `json:"GroupID"`
	Name           string     `json:"Name"`
	AbsolutePath   string     `json:"AbsolutePath"`
	RelativePath   string     `json:"RelativePath"`
	GitURL         *string    `json:"GitURL"`
	Remark         *string    `json:"Remark"`
	LastAccessedAt *time.Time `json:"LastAccessedAt"`
}

func (Project) TableName() string { return "projects" }

// ProjectListItem 项目列表项：指定 groupId 时合并磁盘未认领目录。
//   - Claimed=true：已入库
//   - Claimed=false：仅存于磁盘、尚未认领；其 ID 为负的哨兵值（避免前端 DataTable 行 key 冲突），其余为空
type ProjectListItem struct {
	Project
	Claimed bool `json:"Claimed"`
}

// ── 辅助函数 ──────────────────────────────────────────────────────

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

// ── 写方向回调（注入磁盘副作用，保持基线纯 CRUD 不被污染）──────────────

// beforeCreateGroup 建组前校验名称/根目录并创建根目录。
func beforeCreateGroup(row any) error {
	g := row.(*ProjectGroup)
	if g.Name == "" {
		return fmt.Errorf("项目组名称不能为空")
	}
	if g.AbsolutePath == "" {
		return fmt.Errorf("根目录绝对路径不能为空")
	}
	if err := os.MkdirAll(g.AbsolutePath, 0o755); err != nil {
		return fmt.Errorf("根目录不存在且创建失败: %s", err.Error())
	}
	g.AbsolutePath = filepath.Clean(g.AbsolutePath)
	return nil
}

// afterDeleteGroup 删组（软删）后级联软删其子项目。
func afterDeleteGroup(row any) error {
	g := row.(*ProjectGroup)
	now := time.Now()
	if err := config.GetDB().Model(&Project{}).
		Where("group_id = ? AND is_deleted = ?", g.ID, false).
		Updates(map[string]interface{}{"is_deleted": true, "updated_at": now}).Error; err != nil {
		return err
	}
	return nil
}

// afterUpdateGroup 组根目录变更后重算各子项目的绝对路径。
func afterUpdateGroup(row any) error {
	g := row.(*ProjectGroup)
	var children []Project
	if err := config.GetDB().Where("group_id = ? AND is_deleted = ?", g.ID, false).Find(&children).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, c := range children {
		newAbs := filepath.Join(g.AbsolutePath, c.RelativePath)
		if newAbs != c.AbsolutePath {
			if err := config.GetDB().Model(&c).
				Updates(map[string]interface{}{"absolute_path": newAbs, "updated_at": now}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// beforeCreateProject 建项目前解析路径、校验目录存在、补全名称与访问时间。
func beforeCreateProject(row any) error {
	p := row.(*Project)
	if p.GroupID == 0 {
		return fmt.Errorf("所属项目组ID不能为空")
	}
	var group ProjectGroup
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", p.GroupID, false).First(&group).Error; err != nil {
		return fmt.Errorf("所属项目组不存在")
	}
	abs, rel, err := resolveProjectPaths(group, p.AbsolutePath, p.RelativePath)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return fmt.Errorf("项目目录不存在: %s", abs)
	}
	p.AbsolutePath = abs
	p.RelativePath = rel
	if p.Name == "" {
		p.Name = filepath.Base(abs)
	}
	createdAt := DirCreatedAt(abs)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	p.CreatedAt = createdAt
	last := createdAt
	p.LastAccessedAt = &last
	return nil
}

// afterDeleteProject 删项目（软删）后清空物理目录；物理删除失败仅记录，DB 记录照常软删。
func afterDeleteProject(row any) error {
	p := row.(*Project)
	if err := RemoveDirSafe(p.AbsolutePath); err != nil {
		// 与历史行为一致：DB 记录已删除，仅物理目录残留，不阻断流程。
		fmt.Printf("项目物理目录删除失败（已软删记录）：%s: %v\n", p.AbsolutePath, err)
	}
	return nil
}

// ── 列表（派生：合并已认领 + 磁盘未认领）────────────────────────────

// listProjects 自定义列表：按 groupId 过滤，合并磁盘扫描出的未认领子目录。
func listProjects(c *gin.Context) {
	q := pagination.Parse(c)
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
	if name := c.Query("name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}

	var projects []Project
	if err := db.Order("last_accessed_at DESC NULLS LAST, created_at DESC, id DESC").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	var items []ProjectListItem
	if groupID != nil {
		var group ProjectGroup
		if err := config.GetDB().Where("id = ? AND is_deleted = ?", *groupID, false).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目组不存在"})
			return
		}
		items = buildProjectList(group, projects)
	} else {
		for _, p := range projects {
			items = append(items, ProjectListItem{Project: p, Claimed: true})
		}
	}

	start := q.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + q.Size
	if end > len(items) {
		end = len(items)
	}
	c.JSON(http.StatusOK, pagination.New(items[start:end], int64(len(items)), q))
}

// buildProjectList 把已认领项目与磁盘未认领子目录合并；未认领项用负哨兵 ID 避免行 key 冲突。
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
		item := ProjectListItem{
			Project: Project{
				GroupID:      group.ID,
				Name:         e.Name(),
				AbsolutePath: abs,
				RelativePath: rel,
			},
			Claimed: false,
		}
		// 负哨兵 ID：避免与已认领项（正 ID）冲突导致前端 DataTable 行 key 重复
		item.ID = -(len(unclaimedList) + 1)
		unclaimedList = append(unclaimedList, unclaimed{item: item, name: e.Name()})
	}

	sort.Slice(unclaimedList, func(i, j int) bool {
		return unclaimedList[i].name < unclaimedList[j].name
	})
	for _, u := range unclaimedList {
		items = append(items, u.item)
	}
	return items
}

// ── 扩展动作（基线之外，自定义路由）────────────────────────────────

// MoveProject 移动项目文件夹（同卷 rename / 跨卷 copy）并同步路径字段。
func MoveProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "移动文件夹失败: " + err.Error()})
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

	config.GetDB().First(&project, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "移动成功", "moved": moved, "project": project,
		"oldAbsPath": oldAbs, "newAbsPath": newAbs,
	})
}

// AccessProject 记录访问时间。
func AccessProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的项目ID"})
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

// ── 路由注册（自包含）────────────────────────────────────────────

// Register 把项目管理两套资源的路由挂载到给定路由组。
// 标准 CRUD 交给通用 crud；项目列表（合并未认领目录）与移动/访问为扩展能力，自定义挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "projectGroups", &ProjectGroup{}, crud.Opts{
		Searchable:  []string{"name"},
		Sortable:    []string{"order_num", "created_at", "id"},
		BeforeCreate: beforeCreateGroup,
		AfterUpdate:  afterUpdateGroup,
		AfterDelete:  afterDeleteGroup,
	})

	crud.Register(rg, "projects", &Project{}, crud.Opts{
		Searchable: []string{"name"},
		Sortable:   []string{"last_accessed_at", "created_at", "id"},
		// 路径类字段只能经带副作用的专属流程（建项目 / 移动）改写，禁止通用 PATCH 绕过。
		Protected:    []string{"GroupID", "AbsolutePath", "RelativePath", "LastAccessedAt", "CreatedAt"},
		BeforeCreate: beforeCreateProject,
		AfterDelete:  afterDeleteProject,
		ListHandler:  listProjects,
	})

	projects := rg.Group("/projects")
	projects.PATCH("/:id/move", MoveProject)
	projects.PATCH("/:id/access", AccessProject)
}
