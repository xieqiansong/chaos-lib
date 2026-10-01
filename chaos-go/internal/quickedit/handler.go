package quickedit

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

const maxContentLength = 10 * 1024 * 1024

// findFileByID 按 ID 加载文件，不存在时直接返回 404 并置 ok=false。
func findFileByID(c *gin.Context, id int) (*QuickEditFile, bool) {
	file, err := FindFileByID(id)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "文件不存在")
		return nil, false
	}
	return file, true
}

// buildFileResponse 富化文件响应：补上最近一次快照信息。
func buildFileResponse(file QuickEditFile) QuickEditFileResponse {
	resp := QuickEditFileResponse{
		ID:        file.ID,
		Name:      file.Name,
		FilePath:  file.FilePath,
		Remark:    file.Remark,
		CreatedAt: file.CreatedAt,
		UpdatedAt: file.UpdatedAt,
	}
	if latest, err := GetLatestSnapshot(file.ID); err == nil && latest.ID > 0 {
		resp.LastSnapshotID = latest.ID
		resp.LastSnapshotTime = latest.CreatedAt
	}
	return resp
}

// ── Handlers ──────────────────────────────────────────────────────

func ListQuickEdits(c *gin.Context) {
	files, err := ListFiles()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	responses := make([]QuickEditFileResponse, 0, len(files))
	for _, f := range files {
		responses = append(responses, buildFileResponse(f))
	}
	renv.Success(c, responses)
}

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
	absPath, err := filepath.Abs(req.FilePath)
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "路径无效: "+err.Error())
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		renv.Error(c, http.StatusBadRequest, "文件路径不存在")
		return
	}
	if _, err := FindFileByPath(absPath); err == nil {
		renv.Error(c, http.StatusBadRequest, "该文件已在管控列表中")
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}
	if len(data) > maxContentLength {
		renv.Error(c, http.StatusBadRequest, "文件过大，暂不支持")
		return
	}
	name := req.Name
	if name == "" {
		name = filepath.Base(absPath)
	}
	file := QuickEditFile{
		Name:      name,
		FilePath:  absPath,
		Remark:    req.Remark,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := CreateFile(&file); err != nil {
		renv.Error(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	snapshot, err := TakeSnapshot(file.ID, string(data))
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "文件已登记，但快照失败: "+err.Error())
		return
	}
	resp := buildFileResponse(file)
	renv.Success(c, gin.H{
		"message":         "创建成功",
		"data":            resp,
		"firstSnapshotId": snapshot.ID,
	})
}

func DeleteQuickEdit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	if _, err := FindFileByID(id); err != nil {
		renv.Error(c, http.StatusNotFound, "文件不存在")
		return
	}
	if err := DeleteFile(id); err != nil {
		renv.Error(c, http.StatusInternalServerError, "删除失败: " + err.Error())
		return
	}
	renv.Success(c, nil)
}

func GetQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	file, ok := findFileByID(c, id)
	if !ok {
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvReadContent == nil {
			renv.Error(c, http.StatusNotImplemented, "EnvReadContent 回调未初始化")
			return
		}
		content, size, err := EnvReadContent()
		if err != nil {
			renv.Error(c, http.StatusInternalServerError, "读取环境变量失败: " + err.Error())
			return
		}
		renv.Success(c, gin.H{"content": content, "filePath": file.FilePath, "sizeBytes": size})
		return
	}
	data, err := os.ReadFile(file.FilePath)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "读取文件失败: " + err.Error())
		return
	}
	renv.Success(c, gin.H{"content": string(data), "filePath": file.FilePath, "sizeBytes": len(data)})
}

func UpdateQuickEditContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	file, ok := findFileByID(c, id)
	if !ok {
		return
	}
	var req struct{ Content string }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len([]byte(req.Content)) > maxContentLength {
		renv.Error(c, http.StatusBadRequest, "内容过大，暂不支持")
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvWriteContent == nil {
			renv.Error(c, http.StatusNotImplemented, "EnvWriteContent 回调未初始化")
			return
		}
		warnings, writeErr := EnvWriteContent(req.Content)
		if writeErr != nil {
			warnings = append(warnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnap, snapErr := TakeSnapshot(file.ID, req.Content)
		if snapErr != nil {
			renv.Error(c, http.StatusInternalServerError, "环境变量已更新，但快照失败: "+snapErr.Error())
			return
		}
		file.UpdatedAt = time.Now()
		_ = UpdateFileUpdatedAt(file.ID, file.UpdatedAt)
		resp := buildFileResponse(*file)
		renv.Success(c, gin.H{"message": "环境变量已更新", "data": resp, "snapshotId": afterSnap.ID, "snapshotTime": afterSnap.CreatedAt, "warnings": warnings})
		return
	}
	if err := os.WriteFile(file.FilePath, []byte(req.Content), 0644); err != nil {
		renv.Error(c, http.StatusInternalServerError, "写入文件失败 (可能需要管理员权限): " + err.Error())
		return
	}
	afterSnap, snapErr := TakeSnapshot(file.ID, req.Content)
	if snapErr != nil {
		renv.Error(c, http.StatusInternalServerError, "文件已更新，但快照失败: " + snapErr.Error())
		return
	}
	file.UpdatedAt = time.Now()
	_ = UpdateFileUpdatedAt(file.ID, file.UpdatedAt)
	resp := buildFileResponse(*file)
	renv.Success(c, gin.H{"message": "更新成功", "data": resp, "snapshotId": afterSnap.ID, "snapshotTime": afterSnap.CreatedAt})
}

func ListQuickEditSnapshots(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	if _, ok := findFileByID(c, id); !ok {
		return
	}
	q := pagination.Parse(c)
	snaps, total, err := ListSnapshots(id, q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: " + err.Error())
		return
	}
	items := make([]QuickEditSnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		items = append(items, QuickEditSnapshotResponse{ID: s.ID, FileID: s.FileID, SizeBytes: s.SizeBytes, CreatedAt: s.CreatedAt})
	}
	renv.Success(c, pagination.New(items, total, q))
}

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
	if _, ok := findFileByID(c, fileID); !ok {
		return
	}
	snap, err := FindSnapshot(fileID, snapID)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "快照不存在")
		return
	}
	renv.Success(c, gin.H{"id": snap.ID, "fileId": snap.FileID, "content": snap.Content, "sizeBytes": snap.SizeBytes, "createdAt": snap.CreatedAt})
}

func RestoreQuickEdit(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的 id")
		return
	}
	file, ok := findFileByID(c, fileID)
	if !ok {
		return
	}
	var req struct{ SnapshotID int }
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	snap, err := FindSnapshot(fileID, req.SnapshotID)
	if err != nil {
		renv.Error(c, http.StatusNotFound, "快照不存在")
		return
	}
	if isEnvVirtualFile(file.FilePath) {
		if EnvWriteContent == nil {
			renv.Error(c, http.StatusNotImplemented, "EnvWriteContent 回调未初始化")
			return
		}
		writeWarnings, writeErr := EnvWriteContent(snap.Content)
		if writeErr != nil {
			writeWarnings = append(writeWarnings, fmt.Sprintf("写入异常: %v", writeErr))
		}
		afterSnap, snapErr := TakeSnapshot(fileID, snap.Content)
		if snapErr != nil {
			writeWarnings = append(writeWarnings, "新快照失败: "+snapErr.Error())
		}
		file.UpdatedAt = time.Now()
		_ = UpdateFileUpdatedAt(file.ID, file.UpdatedAt)
		resp := buildFileResponse(*file)
		renv.Success(c, gin.H{"message": "环境变量已回滚", "data": resp, "fromSnapshotId": snap.ID, "snapshotId": afterSnap.ID, "warnings": writeWarnings})
		return
	}
	if err := os.WriteFile(file.FilePath, []byte(snap.Content), 0644); err != nil {
		renv.Error(c, http.StatusInternalServerError, "写入文件失败 (可能需要管理员权限): " + err.Error())
		return
	}
	afterSnap, snapErr := TakeSnapshot(fileID, snap.Content)
	if snapErr != nil {
		renv.Error(c, http.StatusInternalServerError, "文件已回滚，但快照失败: " + snapErr.Error())
		return
	}
	file.UpdatedAt = time.Now()
	_ = UpdateFileUpdatedAt(file.ID, file.UpdatedAt)
	resp := buildFileResponse(*file)
	renv.Success(c, gin.H{"message": "回滚成功", "data": resp, "fromSnapshotId": snap.ID, "snapshotId": afterSnap.ID})
}

// ── 路由注册（自包含）────────────────────────────────────────────

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("quickEdit", Register)
}

// Register 把 quickedit 全部路由挂载到给定路由组。
// 注意：本模块未走通用 crud，由专用 handler 直接接线，以承载快照/虚拟文件等定制行为。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/quickEdits")
	g.GET("/", ListQuickEdits)
	g.POST("/", CreateQuickEdit)
	g.DELETE("/:id", DeleteQuickEdit)
	g.GET("/:id/content", GetQuickEditContent)
	g.PUT("/:id/content", UpdateQuickEditContent)
	g.GET("/:id/snapshots", ListQuickEditSnapshots)
	g.GET("/:id/snapshots/:snapshotId", GetQuickEditSnapshot)
	g.POST("/:id/restore", RestoreQuickEdit)
}
