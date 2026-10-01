package project

import (
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/crud"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("project", Register)
}

// Register 把项目管理两套资源的路由挂载到给定路由组。
// 标准 CRUD 交给通用 crud（回调只做转发，实现见 service.go）；
// 项目列表（合并未认领目录）与移动 / 访问为扩展能力，自定义挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register[ProjectGroup](rg, "projectGroups", crud.Opts[ProjectGroup]{
		Searchable:   []string{"name"},
		Sortable:     []string{"order_num", "created_at", "id"},
		BeforeCreate: ValidateGroupForCreate,
		AfterUpdate:  RelocateProjectsAfterGroupUpdate,
		AfterDelete:  CascadeDeleteProjects,
	})

	crud.Register[Project](rg, "projects", crud.Opts[Project]{
		Searchable: []string{"name"},
		Sortable:   []string{"last_accessed_at", "created_at", "id"},
		// 路径类字段只能经带副作用的专属流程（建项目 / 移动）改写，禁止通用 PATCH 绕过。
		Protected:    []string{"GroupID", "AbsolutePath", "RelativePath", "LastAccessedAt", "CreatedAt"},
		BeforeCreate: PrepareProjectForCreate,
		AfterDelete:  RemoveProjectDir,
		ListHandler:  listProjects,
	})

	projects := rg.Group("/projects")
	projects.PATCH("/:id/move", moveProject)
	projects.PATCH("/:id/access", accessProject)
}

// ── 自定义列表 ──────────────────────────────────────────────────

// listProjects 自定义列表：按 groupId 过滤，合并磁盘扫描出的未认领子目录（合并逻辑在 service）。
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

	items, err := ListProjectItems(groupID, c.Query("name"))
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			renv.Error(c, http.StatusNotFound, "项目组不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
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

// ── 扩展动作 ────────────────────────────────────────────────────

// moveProject 移动项目文件夹并同步路径字段（PATCH /projects/:id/move）。
func moveProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的项目ID")
		return
	}
	var req MoveProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.TargetGroupID == 0 {
		renv.Error(c, http.StatusBadRequest, "目标项目组ID不能为空")
		return
	}
	if err := MoveProject(id, req); err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			renv.Error(c, http.StatusNotFound, "项目不存在")
		case errors.Is(err, ErrGroupNotFound):
			renv.Error(c, http.StatusNotFound, "目标项目组不存在")
		case errors.Is(err, ErrInvalidPath):
			renv.Error(c, http.StatusBadRequest, err.Error())
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	renv.Success(c, nil)
}

// accessProject 记录项目访问时间（PATCH /projects/:id/access）。
func accessProject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的项目ID")
		return
	}
	if err := RecordAccess(id); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			renv.Error(c, http.StatusNotFound, "项目不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, "更新访问时间失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}
