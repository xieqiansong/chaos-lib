package project

import (
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

var projectGroupOpts = crud.Opts[ProjectGroup]{
	Searchable:   []string{"name"},
	Sortable:     []string{"order_num", "created_at", "id"},
	BeforeCreate: ValidateGroupForCreate,
	AfterUpdate:  RelocateProjectsAfterGroupUpdate,
	AfterDelete:  CascadeDeleteProjects,
}
var projectOpts = crud.Opts[Project]{
	Searchable: []string{"name"},
	Sortable:   []string{"last_accessed_at", "created_at", "id"},

	Protected:    []string{"GroupID", "AbsolutePath", "RelativePath", "LastAccessedAt", "CreatedAt"},
	BeforeCreate: PrepareProjectForCreate,
	AfterDelete:  RemoveProjectDir,

	V1ListHandler: listProjects,
}

func RegisterV1(rg *web.RouterGroup) {
	crud.RegisterActions[ProjectGroup](rg, "project-groups", projectGroupOpts, nil)
	projects := crud.RegisterActions[Project](rg, "projects", projectOpts, nil)
	projects.POST("/move", projectMoveV1)
	projects.POST("/access", projectAccessV1)
}

func listProjects(c *web.Context) {
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

func moveProject(c *web.Context) {
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

func accessProject(c *web.Context) {
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

func projectMoveV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req MoveProjectRequest
	if err := envelope.Bind(c, &req); err != nil {
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

func projectAccessV1(c *web.Context) {
	id, ok := envelope.ID(c)
	if !ok {
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
