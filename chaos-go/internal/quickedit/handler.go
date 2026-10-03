package quickedit

import (
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("quick-edits", Register)
}

// Register 把 quickedit 全部路由挂载到给定路由组。
// 注意：本模块未走通用 crud，由专用 handler 直接接线，以承载快照/虚拟文件等定制行为。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/quick-edits")
	g.GET("/", ListQuickEdits)
	g.POST("/", CreateQuickEdit)
	g.DELETE("/:id", DeleteQuickEdit)
	g.GET("/:id/content", GetQuickEditContent)
	g.PUT("/:id/content", UpdateQuickEditContent)
	g.GET("/:id/snapshots", ListQuickEditSnapshots)
	g.GET("/:id/snapshots/:snapshotId", GetQuickEditSnapshot)
	g.POST("/:id/restore", RestoreQuickEdit)
}

// ListQuickEdits 列出受管控文件（含最近一次快照信息）。
func ListQuickEdits(c *gin.Context) {
	items, err := ListFilesWithSnapshot()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, items)
}

// CreateQuickEdit 登记一个受管控文件并落首条快照。
func CreateQuickEdit(c *gin.Context) {
	var req struct {
		Name     string
		FilePath string
		Remark   string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	file, snapshot, err := RegisterFile(req.Name, req.FilePath, req.Remark)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileExists), errors.Is(err, ErrPathNotFound),
			errors.Is(err, ErrInvalidPath), errors.Is(err, ErrContentTooLarge):
			renv.Error(c, http.StatusBadRequest, err.Error())
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	resp := buildFileResponse(*file)
	renv.Success(c, gin.H{
		"message":         "创建成功",
		"data":            resp,
		"firstSnapshotId": snapshot.ID,
	})
}

// DeleteQuickEdit 删除受管控文件记录（不动磁盘文件）。
func DeleteQuickEdit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	if err := DeleteFileByID(id); err != nil {
		if errors.Is(err, ErrFileNotFound) {
			renv.Error(c, http.StatusNotFound, "文件不存在")
			return
		}
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, nil)
}

// GetQuickEditContent 读取文件当前内容（虚拟文件走 envvar 回调）。
func GetQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	view, err := ReadContent(id)
	if err != nil {
		writeError(c, err)
		return
	}
	renv.Success(c, view)
}

// UpdateQuickEditContent 保存内容并追加一条快照。
func UpdateQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	var req struct{ Content string }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	res, err := SaveContent(id, req.Content)
	if err != nil {
		writeError(c, err)
		return
	}
	renv.Success(c, saveResultToMap(res, false))
}

// ListQuickEditSnapshots 分页列出某文件的历史快照。
func ListQuickEditSnapshots(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	q := pagination.Parse(c)
	items, total, err := ListSnapshotsOf(id, q)
	if err != nil {
		writeError(c, err)
		return
	}
	renv.Success(c, pagination.New(items, total, q))
}

// GetQuickEditSnapshot 读取单条快照内容。
func GetQuickEditSnapshot(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 file id")
		return
	}
	snapID, err := strconv.Atoi(c.Param("snapshotId"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 snapshot id")
		return
	}
	snap, err := GetSnapshot(fileID, snapID)
	if err != nil {
		writeError(c, err)
		return
	}
	renv.Success(c, gin.H{
		"id":        snap.ID,
		"fileId":    snap.FileID,
		"content":   snap.Content,
		"sizeBytes": snap.SizeBytes,
		"createdAt": snap.CreatedAt,
	})
}

// RestoreQuickEdit 回滚到指定快照，并追加一条新快照。
func RestoreQuickEdit(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	var req struct{ SnapshotID int }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	res, err := RestoreSnapshot(fileID, req.SnapshotID)
	if err != nil {
		writeError(c, err)
		return
	}
	renv.Success(c, saveResultToMap(res, true))
}

// ── 响应辅助 ────────────────────────────────────────────────────

// writeError 把 service 返回的领域错误映射为 HTTP 状态码。
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrFileNotFound):
		renv.Error(c, http.StatusNotFound, "文件不存在")
	case errors.Is(err, ErrSnapshotNotFound):
		renv.Error(c, http.StatusNotFound, "快照不存在")
	case errors.Is(err, ErrEnvNotReady):
		renv.Error(c, http.StatusNotImplemented, err.Error())
	case errors.Is(err, ErrContentTooLarge):
		renv.Error(c, http.StatusBadRequest, err.Error())
	default:
		renv.Error(c, http.StatusInternalServerError, err.Error())
	}
}

// saveResultToMap 把保存 / 回滚结果转为响应体；withFrom 时附带来源快照 id。
func saveResultToMap(res *SaveResult, withFrom bool) gin.H {
	out := gin.H{
		"message":      res.Message,
		"data":         res.Data,
		"snapshotId":   res.SnapshotID,
		"snapshotTime": res.SnapshotTime,
	}
	if withFrom {
		out["fromSnapshotId"] = res.FromSnapshotID
	}
	if len(res.Warnings) > 0 {
		out["warnings"] = res.Warnings
	}
	return out
}
