// Package filelink 管理本地文件/目录联接点（Windows junction / POSIX symlink）。
//
// 参考标准数据（internal/standarddata）实现：纯 CRUD 交给通用 crud，业务特有的部分用回调注入：
//   - ToResponse：派生字段 LinkStatus 按文件系统实时计算（不落库，返回时重新算）
//   - AfterCreate：校验源/目标路径，不合法则不落库
//   - AfterDelete：清理联接点，清理失败则整笔软删回滚
//
// 启用开关有副作用（建/删联接点）且带校验，故保留自定义子路由 PATCH /fileLinks/:id/status，
// 与标准数据保持同一套「标准之下的自定义」范式。
package filelink

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaos-go/config"
	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

// FileLink 文件连接。嵌入 crud.BaseModel 获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type FileLink struct {
	crud.BaseModel
	SourcePath string `json:"SourcePath"`
	TargetPath string `json:"TargetPath"`
	Status     bool   `json:"Status"`
	Remark     string `json:"Remark"`
	Sort       int    `gorm:"default:0" json:"Sort"`
}

// TableName 显式指定表名。
func (FileLink) TableName() string { return "file_links" }

// FileLinkResponse 对外响应：模型字段之外附带实时计算的 LinkStatus。
type FileLinkResponse struct {
	ID         int       `json:"ID"`
	SourcePath string    `json:"SourcePath"`
	TargetPath string    `json:"TargetPath"`
	Status     bool      `json:"Status"`
	Remark     string    `json:"Remark"`
	Sort       int       `json:"Sort"`
	LinkStatus string    `json:"LinkStatus"`
	CreatedAt  time.Time `json:"CreatedAt"`
	UpdatedAt  time.Time `json:"UpdatedAt"`
}

// ── 辅助 ──────────────────────────────────────────────────────────

// normalizeLinkPath 去掉 os.Readlink 返回的卷命名空间前缀（\??\ 或 \\?\），
// 还原为普通盘符路径，便于与源路径比较。
func normalizeLinkPath(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + p[len(`\\?\UNC\`):]
	}
	if strings.HasPrefix(p, `\??\UNC\`) {
		return `\\` + p[len(`\??\UNC\`):]
	}
	if strings.HasPrefix(p, `\??\`) || strings.HasPrefix(p, `\\?\`) {
		return p[4:]
	}
	return p
}

// checkLinkStatus 按文件系统真实状态推导连接状态（不落库、每次读时重算）：
// normal 正常 / missing 目标缺失 / none 未启用 / invalid 无效 / conflict 冲突。
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
		absActual, _ := filepath.Abs(normalizeLinkPath(actualTarget))
		absSource, _ := filepath.Abs(sourcePath)
		if strings.EqualFold(absActual, absSource) {
			return "normal"
		}
	}
	return "conflict"
}

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
func toResponse(rows any) any {
	links := rows.([]*FileLink)
	out := make([]FileLinkResponse, 0, len(links))
	for _, l := range links {
		out = append(out, toOne(l))
	}
	return out
}

// ── 注册 ──────────────────────────────────────────────────────────

// Register 把本资源的路由挂载到给定路由组。
// 纯 CRUD 交给通用 crud；状态切换在基线之外由本包自实现并挂载。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "fileLinks", &FileLink{}, crud.Opts{
		Searchable: []string{"source_path", "target_path", "remark"},
		Sortable:   []string{"id", "sort"},
		// Status 只能走 /:id/status（带建删联接点副作用），禁止通用 PATCH 改写
		Protected:  []string{"status"},
		ToResponse: toResponse,
		AfterCreate: func(row any) error {
			l := row.(*FileLink)
			if strings.TrimSpace(l.SourcePath) == "" || strings.TrimSpace(l.TargetPath) == "" {
				return fmt.Errorf("源路径和目标路径不能为空")
			}
			if _, err := os.Stat(l.SourcePath); err != nil {
				return fmt.Errorf("源路径不存在: %s", l.SourcePath)
			}
			return nil
		},
		AfterDelete: func(row any) error {
			l := row.(*FileLink)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := config.GetDB().Begin()
	var link FileLink
	if err := tx.Where("is_deleted = ?", false).First(&link, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "文件连接不存在"})
		return
	}

	if req.Status {
		if link.Status {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件连接已启用"})
			return
		}
		if _, err := os.Stat(link.SourcePath); os.IsNotExist(err) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "源路径不存在"})
			return
		}
		if _, err := os.Lstat(link.TargetPath); err == nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "目标路径已存在，请先删除或移动"})
			return
		}
		if err := CreateJunction(link.SourcePath, link.TargetPath); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录联接点失败: " + err.Error()})
			return
		}
		link.Status = true
	} else {
		if !link.Status {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件连接已禁用"})
			return
		}
		if err := os.Remove(link.TargetPath); err != nil && !os.IsNotExist(err) {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除联接点失败: " + err.Error()})
			return
		}
		link.Status = false
	}

	if err := tx.Model(&link).Update("status", link.Status).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "状态更新失败: " + err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "状态更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "状态更新成功", "data": toOne(&link)})
}
