package project

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"
	"chaos-go/pkg/tools"

	"github.com/gin-gonic/gin"
)

// ── 写方向回调（注入磁盘副作用，保持基线纯 CRUD 不被污染）──────────────

// beforeCreateGroup 建组前校验名称/根目录并创建根目录。
func beforeCreateGroup(row *ProjectGroup) error {
	g := row
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
func afterDeleteGroup(row *ProjectGroup) error {
	g := row
	return CascadeDeleteProjectsByGroup(g.ID)
}

// afterUpdateGroup 组根目录变更后重算各子项目的绝对路径。
func afterUpdateGroup(row *ProjectGroup) error {
	g := row
	children, err := FindActiveProjectsByGroup(g.ID)
	if err != nil {
		return err
	}
	for _, c := range children {
		newAbs := filepath.Join(g.AbsolutePath, c.RelativePath)
		if newAbs != c.AbsolutePath {
			if err := UpdateProjectAbsolutePath(c.ID, newAbs); err != nil {
				return err
			}
		}
	}
	return nil
}

// beforeCreateProject 建项目前解析路径、校验目录存在、补全名称与访问时间。
func beforeCreateProject(row *Project) error {
	p := row
	if p.GroupID == 0 {
		return fmt.Errorf("所属项目组ID不能为空")
	}
	group, err := FindGroupByID(p.GroupID)
	if err != nil {
		return fmt.Errorf("所属项目组不存在")
	}
	abs, rel, err := resolveProjectPaths(*group, p.AbsolutePath, p.RelativePath)
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
	createdAt := tools.DirCreatedAt(abs)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	p.CreatedAt = createdAt
	last := createdAt
	p.LastAccessedAt = &last
	return nil
}

// afterDeleteProject 删项目（软删）后清空物理目录；物理删除失败仅记录，DB 记录照常软删。
func afterDeleteProject(row *Project) error {
	p := row
	if err := tools.RemoveDirSafe(p.AbsolutePath); err != nil {
		// 与历史行为一致：DB 记录已删除，仅物理目录残留，不阻断流程。
		fmt.Printf("项目物理目录删除失败（已软删记录）：%s: %v\n", p.AbsolutePath, err)
	}
	return nil
}

// ── 列表（派生：合并已认领 + 磁盘未认领）────────────────────────────

// listProjects 自定义列表：按 groupId 过滤，合并磁盘扫描出的未认领子目录。
func listProjects(c *gin.Context) {
	q := pagination.Parse(c)
	var groupID *int
	if gid := c.Query("groupId"); gid != "" {
		id, err := strconv.Atoi(gid)
		if err != nil {
			renv.Error(c, http.StatusBadRequest, "无效的groupId")
			return
		}
		groupID = &id
	}
	name := c.Query("name")

	projects, err := ListActiveProjects(groupID, name)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	var items []ProjectListItem
	if groupID != nil {
		group, err := FindGroupByID(*groupID)
		if err != nil {
			renv.Error(c, http.StatusNotFound, "项目组不存在")
			return
		}
		items = buildProjectList(*group, projects)
	} else {
		for _, p := range projects {
			items = append(items, ProjectListItem{Project: p, Claimed: true})
		}
	}

	start := q.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + q.PageSize
	if end > len(items) {
		end = len(items)
	}
	renv.Success(c, pagination.New(items[start:end], int64(len(items)), q))
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
		renv.Error(c, http.StatusBadRequest, "无效的项目ID")
		return
	}
	var req struct {
		TargetGroupID      int    `json:"TargetGroupID"`
		TargetRelativePath string `json:"TargetRelativePath"`
		TargetAbsPath      string `json:"TargetAbsPath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.TargetGroupID == 0 {
		renv.Error(c, http.StatusBadRequest, "目标项目组ID不能为空")
		return
	}

	project, err := FindProjectByID(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "项目不存在")
		return
	}
	group, err := FindGroupByID(req.TargetGroupID)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "目标项目组不存在")
		return
	}

	newAbs, newRel, err := resolveProjectPaths(*group, req.TargetAbsPath, req.TargetRelativePath)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	oldAbs := project.AbsolutePath
	moved := newAbs != oldAbs

	if moved {
		// tools.MoveProjectFolder 仅复制、不删源，需显式清理旧目录（与原内部版语义一致：移动后即删除源）
		if err := tools.MoveProjectFolder(oldAbs, newAbs); err != nil {
			renv.Error(c, http.StatusInternalServerError, "移动文件夹失败: "+err.Error())
			return
		}
		if err := tools.RemoveDirSafe(oldAbs); err != nil {
			fmt.Printf("移动后清理旧目录失败（新目录已就位）: %s: %v\n", oldAbs, err)
		}
	}

	if err := UpdateProjectLocation(id, group.ID, newAbs, newRel); err != nil {
		if moved {
			_ = tools.RemoveDirSafe(newAbs)
		}
		renv.Error(c, http.StatusInternalServerError, "更新路径失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

// AccessProject 记录访问时间。
func AccessProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的项目ID")
		return
	}
	if _, err := FindProjectByID(id); err != nil {
		renv.Error(c, http.StatusNotFound, "项目不存在")
		return
	}
	if err := TouchProjectAccess(id); err != nil {
		renv.Error(c, http.StatusInternalServerError, "更新访问时间失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

// ── 路由注册（自包含）────────────────────────────────────────────

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("project", Register)
}

// Register 把项目管理两套资源的路由挂载到给定路由组。
// 标准 CRUD 交给通用 crud；项目列表（合并未认领目录）与移动/访问为扩展能力，自定义挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register[ProjectGroup](rg, "projectGroups", crud.Opts[ProjectGroup]{
		Searchable:   []string{"name"},
		Sortable:     []string{"order_num", "created_at", "id"},
		BeforeCreate: beforeCreateGroup,
		AfterUpdate:  afterUpdateGroup,
		AfterDelete:  afterDeleteGroup,
	})

	crud.Register[Project](rg, "projects", crud.Opts[Project]{
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
