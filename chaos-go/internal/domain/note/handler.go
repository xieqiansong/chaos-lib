package note

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
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
	routehub.RegisterV1("notes", RegisterV1)
}

// noteOpts 标准 CRUD 选项，存量 /api 与 v1 动作路由共用。
// 注意：BeforeCreate 故意返回错误——笔记是文件优先模型，禁止经基线 POST 直接建 DB 行，
// 必须经自定义 createNote（v1 create 动作，见 handler_v1.go）走文件系统创建；
// V1CreateHandler 覆盖基线 createAction，使 v1 的 create 走文件优先路径；
// ListHandler 的目录/树过滤逻辑由 listNotes 提供（v1 经 V1ListHandler 复用）。
var noteOpts = crud.Opts[Note]{
	Searchable: []string{"title", "name", "summary", "search_text"},
	Sortable:   []string{"updated_at", "created_at", "title", "size_bytes"},
	// 笔记是文件优先模型：基线 POST /notes 直接插 DB 行无对应文件，必须禁掉；
	// 派生字段（路径/名称/格式/hash/大小/时间等）禁止通用 PATCH 改写，仅 Starred 可经 PATCH 切换。
	Protected:       []string{"RelPath", "ParentRel", "Name", "Title", "Summary", "SearchText", "Format", "SizeBytes", "ContentHash", "DiskMTime", "IndexedAt", "WordCount", "TagNames"},
	BeforeCreate:    func(*Note) error { return errors.New("请通过文件系统或 POST /notes/create 创建笔记") },
	ToResponse:      ToNoteResponses,
	V1ListHandler:   listNotes,
	V1CreateHandler: noteCreateV1,
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 标准 CRUD（list/get/create/update/delete/batchCreate/batchDelete）由通用 crud 生成，
// 其中 create 由 V1CreateHandler 覆盖为文件优先创建；树 / 内容 / 扫描 / 保存 /
// 重命名 / 移动 / 回收等文件优先动作见 handler_v1.go。
func RegisterV1(rg *gin.RouterGroup) {
	g := crud.RegisterActions[Note](rg, "notes", noteOpts, nil)
	RegisterNoteV1Actions(g)
}

// listNotes 自定义列表：支持目录过滤 + 全文关键词 + 星标 + 磁盘缺失。
func listNotes(c *gin.Context) {
	q := pagination.Parse(c)
	filter := filterFromRequest(c, q)
	notes, total, err := ListNotes(filter)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	renv.Success(c, pagination.New(notes, total, q))
}

// filterFromRequest 把列表筛选参数收敛为 ListFilter。
//
// 参数有两个来源，必须同时兼容：
//   - 存量 RESTful（GET /notes?dir=…）：参数全在 query string；
//   - v1（POST /notes/list）：参数在信封 meta。前端 DataTable 的 search 经
//     useRestApi 统一收进 meta.like，而 envelope.MetaToQuery 只把整个 like map
//     压成一个字符串 query key（like=map[…]）、不会展开成 dir=…/q=…，
//     所以此处 c.Query 恒为空串——早期实现只读 query，导致目录过滤完全失效：
//     无论在哪个目录点击，列表都退化为「根目录直属笔记」，树里点笔记也就永远
//     匹配不到、右侧不打开。
//
// 语义约定：meta.like 中缺失的字段一律回落到 query 读取；而 dir="" 即根目录，
// 与 query 缺失的默认值一致，故前端「空串不传」的做法无需额外处理。
func filterFromRequest(c *gin.Context, q pagination.Query) ListFilter {
	filter := ListFilter{
		Dir:      c.Query("dir"),
		Query:    c.Query("q"),
		Starred:  c.Query("starred") == "true",
		Missing:  c.Query("missing") == "true",
		Offset:   q.Offset(),
		PageSize: q.PageSize,
	}
	like, _ := envelope.GetMetaMap(c)["like"].(map[string]any)
	if v, ok := like["dir"].(string); ok && v != "" {
		filter.Dir = v
	}
	if v, ok := like["q"].(string); ok && v != "" {
		filter.Query = v
	}
	if v, ok := like["starred"].(bool); ok {
		filter.Starred = v
	}
	if v, ok := like["missing"].(bool); ok {
		filter.Missing = v
	}
	return filter
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
