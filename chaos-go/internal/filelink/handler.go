package filelink

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"chaos-go/internal/config"
	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// ── 读方向回调 ────────────────────────────────────────────────────

// toOne 单条转换：附带实时计算的 LinkStatus。
func toOne(l *FileLink) FileLinkResponse {
	return FileLinkResponse{
		ID:         l.ID,
		SourcePath: l.SourcePath,
		TargetPath: l.TargetPath,
		Status:     l.Status,
		Remark:     l.Remark,
		Sort:       l.Sort,
		LinkStatus: checkLinkStatus(l.SourcePath, l.TargetPath, l.Status),
		CreatedAt:  l.CreatedAt,
		UpdatedAt:  l.UpdatedAt,
	}
}

// toResponse 整批转换（crud.Opts.ToResponse）：入参固定 []*FileLink，便于后续批量/并发优化。
func toResponse(rows []*FileLink) any {
	out := make([]FileLinkResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, toOne(l))
	}
	return out
}

// ── 注册 ──────────────────────────────────────────────────────────

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("fileLinks", Register)
}

// Register 把本资源的路由挂载到给定路由组。
// 纯 CRUD 交给通用 crud；状态切换在基线之外由本包自实现并挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register[FileLink](rg, "fileLinks", crud.Opts[FileLink]{
		Searchable: []string{"source_path", "target_path", "remark"},
		Sortable:   []string{"id", "sort"},
		// Status 只能走 /:id/status（带建删联接点副作用），禁止通用 PATCH 改写
		Protected:  []string{"status"},
		ToResponse: toResponse,
		AfterCreate: func(row *FileLink) error {
			l := row
			if strings.TrimSpace(l.SourcePath) == "" || strings.TrimSpace(l.TargetPath) == "" {
				return fmt.Errorf("源路径和目标路径不能为空")
			}
			if _, err := os.Stat(l.SourcePath); err != nil {
				return fmt.Errorf("源路径不存在: %s", l.SourcePath)
			}
			return nil
		},
		AfterDelete: func(row *FileLink) error {
			l := row
			if !l.Status {
				return nil
			}
			// 联接点已不存在视为清理成功（幂等）
			if err := os.Remove(l.TargetPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("清理联接点失败: %v", err)
			}
			return nil
		},
	})
	// 自定义子路由：状态切换（标准 CRUD 之外的本业务实现）
	rg.Group("/fileLinks").PATCH("/:id/status", status)
}

// ── 自定义子路由 ──────────────────────────────────────────────────

// status 状态切换：校验后建/删联接点并落库，任一步失败则回滚（钩子先执行、成功后才提交）。
func status(c *gin.Context) {
	var req struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tx := config.GetDB().Begin()
	var link FileLink
	if err := tx.Where("is_deleted = ?", false).First(&link, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		renv.Error(c, http.StatusNotFound, "文件连接不存在")
		return
	}

	if req.Status {
		if link.Status {
			tx.Rollback()
			renv.Error(c, http.StatusBadRequest, "文件连接已启用")
			return
		}
		if _, err := os.Stat(link.SourcePath); os.IsNotExist(err) {
			tx.Rollback()
			renv.Error(c, http.StatusBadRequest, "源路径不存在")
			return
		}
		if _, err := os.Lstat(link.TargetPath); err == nil {
			tx.Rollback()
			renv.Error(c, http.StatusBadRequest, "目标路径已存在，请先删除或移动")
			return
		}
		if err := CreateJunction(link.SourcePath, link.TargetPath); err != nil {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "创建目录联接点失败: "+err.Error())
			return
		}
		link.Status = true
	} else {
		if !link.Status {
			tx.Rollback()
			renv.Error(c, http.StatusBadRequest, "文件连接已禁用")
			return
		}
		if err := os.Remove(link.TargetPath); err != nil && !os.IsNotExist(err) {
			tx.Rollback()
			renv.Error(c, http.StatusInternalServerError, "删除联接点失败: "+err.Error())
			return
		}
		link.Status = false
	}

	if err := tx.Model(&link).Update("status", link.Status).Error; err != nil {
		tx.Rollback()
		renv.Error(c, http.StatusInternalServerError, "状态更新失败: "+err.Error())
		return
	}
	if err := tx.Commit().Error; err != nil {
		renv.Error(c, http.StatusInternalServerError, "状态更新失败: "+err.Error())
		return
	}
	renv.Success(c, toOne(&link))
}
