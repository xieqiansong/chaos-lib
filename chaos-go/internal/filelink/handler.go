package filelink

import (
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("file-links", Register)
}

// Register 把本资源的路由挂载到给定路由组。
// 纯 CRUD 交给通用 crud；状态切换在基线之外由本包自实现并挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register[FileLink](rg, "file-links", crud.Opts[FileLink]{
		Searchable: []string{"source_path", "target_path", "remark"},
		Sortable:   []string{"id", "sort"},
		// Status 只能走 /:id/status（带建删联接点副作用），禁止通用 PATCH 改写
		Protected:    []string{"status"},
		ToResponse:   toResponse,
		BeforeCreate: ValidateForCreate,
		AfterDelete:  CleanupLink,
	})
	// 自定义子路由：状态切换（标准 CRUD 之外的本业务实现）
	rg.Group("/file-links").PATCH("/:id/status", status)
}

// status 状态切换：PATCH /file-links/:id/status，body {status:bool}。
func status(c *gin.Context) {
	var req struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	link, err := findActiveByID(id)
	if err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			renv.Error(c, http.StatusNotFound, "文件连接不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := SetStatus(link, req.Status); err != nil {
		switch {
		case errors.Is(err, ErrAlreadyEnabled), errors.Is(err, ErrAlreadyDisabled),
			errors.Is(err, ErrSourceMissing), errors.Is(err, ErrTargetExists):
			renv.Error(c, http.StatusBadRequest, err.Error())
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	renv.Success(c, toOne(link))
}
