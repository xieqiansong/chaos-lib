package note

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// errRules 笔记模块统一的领域错误 → HTTP 状态码映射表。
var errRules = []httpx.ErrRule{
	{Err: ErrNoteNotFound, Status: 404, Msg: "笔记不存在或已被移动"},
	{Err: ErrVaultUnavailable, Status: 400, Msg: "笔记库未配置或不可用"},
	{Err: ErrInvalidPath, Status: 400, Msg: "路径不合法"},
	{Err: ErrPathEscapesVault, Status: 400, Msg: "路径超出笔记库范围"},
	{Err: ErrScanInProgress, Status: 409, Msg: "扫描正在进行中，请稍后再试"},
	{Err: ErrConflict, Status: 409, Msg: "文件已被外部修改，请重新加载或选择覆盖"},
	{Err: ErrAlreadyExists, Status: 409, Msg: "目标位置已存在同名文件"},
	{Err: ErrContentTooLarge, Status: 413, Msg: "文件过大，暂不支持在线编辑"},
}

func init() {
	routehub.Register("notes", Register)
}

// Register 挂载笔记模块路由：标准 CRUD 交给 crud 基线，树 / 内容 / 写操作为自定义扩展。
func Register(rg *gin.RouterGroup) {
	g := crud.Register[Note](rg, "notes", crud.Opts[Note]{
		Searchable: []string{"title", "name", "summary", "search_text"},
		Sortable:   []string{"updated_at", "created_at", "title", "size_bytes"},
		// 笔记是文件优先模型：基线 POST /notes 直接插 DB 行无对应文件，必须禁掉；
		// 派生字段（路径/名称/格式/hash/大小/时间等）禁止通用 PATCH 改写，仅 Starred 可经 PATCH 切换。
		Protected:    []string{"RelPath", "ParentRel", "Name", "Title", "Summary", "SearchText", "Format", "SizeBytes", "ContentHash", "DiskMTime", "IndexedAt", "WordCount", "TagNames"},
		BeforeCreate: func(*Note) error { return errors.New("请通过文件系统或 POST /notes/create 创建笔记") },
		ToResponse:   ToNoteResponses,
		ListHandler:  listNotes,
	})

	g.GET("/tree", getTree)
	g.GET("/:id/content", getContent)
	g.POST("/scan", scanNotes)
	g.POST("/create", createNote)
	g.PUT("/:id/content", putContent)
	g.POST("/:id/rename", renameNote)
	g.POST("/:id/move", moveNote)
	g.POST("/:id/delete", trashNote)
}

// listNotes 自定义列表：支持目录过滤 + 全文关键词 + 星标 + 磁盘缺失。
func listNotes(c *gin.Context) {
	q := pagination.Parse(c)
	filter := ListFilter{
		Dir:      c.Query("dir"),
		Query:    c.Query("q"),
		Starred:  c.Query("starred") == "true",
		Missing:  c.Query("missing") == "true",
		Offset:   q.Offset(),
		PageSize: q.PageSize,
	}
	notes, total, err := ListNotes(filter)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(notes, total, q))
}

// getTree 返回目录树。
func getTree(c *gin.Context) {
	nodes, err := Tree()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "构建目录树失败: "+err.Error())
		return
	}
	renv.Success(c, nodes)
}

// getContent 按 id 读取笔记磁盘原文（只读）。路径经 resolveSafePath 校验。
func getContent(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
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

// scanNotes 触发一次 Vault 扫描（full=true 强制全量重算）。
func scanNotes(c *gin.Context) {
	full := c.Query("full") == "true"
	res, err := ScanVault(full)
	if err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, res)
}

// createNote 新建空笔记文件并建索引（走专属路由，绕过基线 POST 直接插 DB 行）。
func createNote(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

// putContent 保存笔记正文（带 baseHash 乐观锁）；冲突时返回 409 + conflict 详情。
func putContent(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	var req SaveContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	res, conflict, err := SaveContent(id, req.Content, req.BaseHash)
	if err != nil {
		if conflict != nil {
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

// renameNote 重命名笔记文件（仅改文件名）。
func renameNote(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

// moveNote 把笔记移动到目标目录。
func moveNote(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	var req MoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

// trashNote 把笔记移入回收站（.trash），并移除索引行。
func trashNote(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	if err := TrashNote(id); err != nil {
		httpx.MapError(c, err, errRules, http.StatusInternalServerError)
		return
	}
	renv.Success(c, gin.H{"ok": true})
}
