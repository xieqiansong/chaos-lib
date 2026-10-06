package note

// v1 动作路由（POST + Action，详见仓库根《接口规范.md》）。
//
// 笔记是「文件优先」模型：正文与文件名以磁盘为准，DB 仅为派生索引。因此基线 crud 的
// 通用 create / delete 不足以覆盖，v1 在标准 CRUD 之外补齐一组自定义动作，与存量 /api
// 的 tree / content / scan / create / rename / move / delete 语义一一对应，便于删除旧路由。
//
// 主键统一经 envelope.ID 从信封 data.id 取得（v1 路径不带 :id），请求体经 envelope.Bind
// 从信封 data 绑定；查询参数（full 等）经 envelope.MetaToQuery 从 meta 桥接。

import (
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// RegisterNoteV1Actions 在 v1 的 /notes 组下挂载笔记的自定义动作。
// 由 RegisterV1 在 crud.RegisterActions 之后调用。
func RegisterNoteV1Actions(g *gin.RouterGroup) {
	g.POST("/tree", noteTreeV1)
	g.POST("/getContent", noteGetContentV1)
	g.POST("/scan", noteScanV1)
	g.POST("/saveContent", noteSaveContentV1)
	g.POST("/rename", noteRenameV1)
	g.POST("/move", noteMoveV1)
	g.POST("/trash", noteTrashV1)
}

// noteTreeV1 目录树（POST /api/v1/notes/tree）。无参数，直接复用 service。
func noteTreeV1(c *gin.Context) {
	nodes, err := Tree()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "构建目录树失败: "+err.Error())
		return
	}
	renv.Success(c, nodes)
}

// noteGetContentV1 按 id 读取磁盘原文（POST /api/v1/notes/getContent）。
func noteGetContentV1(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	n, err := FindByID(id)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	content, err := ReadContent(n)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, gin.H{
		"relPath":     n.RelPath,
		"name":        n.Name,
		"format":      n.Format,
		"content":     content,
		"contentHash": sha256hex([]byte(content)),
		"sizeBytes":   n.SizeBytes,
	})
}

// noteScanV1 触发一次 Vault 扫描（POST /api/v1/notes/scan）。full 经 meta 传入。
func noteScanV1(c *gin.Context) {
	var m struct {
		Full bool `json:"full"`
	}
	_ = envelope.GetMeta(c, &m)
	res, err := ScanVault(m.Full)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, res)
}

// noteSaveContentV1 保存正文（带 baseHash 乐观锁，POST /api/v1/notes/saveContent）。
// 冲突时返回真实 HTTP 409 + conflict 详情（前端据此做「覆盖 / 重载」），与存量一致。
func noteSaveContentV1(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req SaveContentRequest
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	res, conflict, err := SaveContent(id, req.Content, req.BaseHash)
	if err != nil {
		if conflict != nil {
			// 冲突是唯一需要真实 409 的场景：前端从 axios 错误的 response.data.conflict 读取详情。
			c.JSON(http.StatusConflict, gin.H{
				"code":     http.StatusConflict,
				"message":  ErrConflict.Error(),
				"conflict": conflict,
			})
			return
		}
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, res)
}

// noteRenameV1 重命名笔记文件（仅改文件名，POST /api/v1/notes/rename）。
func noteRenameV1(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req RenameRequest
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	n, err := RenameNote(id, req.Name)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, toNoteResponse(*n))
}

// noteMoveV1 移动笔记到目标目录（根级传空串，POST /api/v1/notes/move）。
func noteMoveV1(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req MoveRequest
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	n, err := MoveNote(id, req.TargetDir)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, toNoteResponse(*n))
}

// noteTrashV1 移入回收站（POST /api/v1/notes/trash）。
func noteTrashV1(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := TrashNote(id); err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, gin.H{"ok": true})
}

// noteCreateV1 新建空笔记文件并建索引（POST /api/v1/notes/create）。
// 作为 V1CreateHandler 覆盖基线 createAction（基线 BeforeCreate 已禁掉通用插库）。
func noteCreateV1(c *gin.Context) {
	var req CreateRequest
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	n, err := CreateNote(req.ParentRel, req.Name)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, toNoteResponse(*n))
}
