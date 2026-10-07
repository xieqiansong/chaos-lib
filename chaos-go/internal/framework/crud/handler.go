package crud

import (
	"fmt"
	"net/http"
	"strings"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// handler[T] 持有资源选项，按方法分发。
type handler[T any] struct {
	opts Opts[T]
}

func (h *handler[T]) sortable(field string) bool {
	for _, f := range h.opts.Sortable {
		if f == field {
			return true
		}
	}
	return false
}

// viewOne 单条：包成 1 元素切片复用同一个批量回调，再取首元素。
func (h *handler[T]) viewOne(row *T) any {
	if h.opts.ToResponse == nil {
		return row
	}
	return firstElem(h.opts.ToResponse([]*T{row}))
}

// runHook 执行写方向回调；返回 error 时调用方负责回滚并响应。
func (h *handler[T]) runHook(hook func(row *T) error, row *T) error {
	if hook == nil {
		return nil
	}
	return hook(row)
}

// listMeta 信封 meta 中的列表查询参数（对应规范 list 动作）。
type listMeta struct {
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Sort     string         `json:"sort"`
	Order    string         `json:"order"`
	Filter   map[string]any `json:"filter"`
	Keyword  string         `json:"keyword"`
	// Like：字段级模糊搜索（v1 对应前端 DataTable 的 search），仅对 Searchable 列生效，
	// 与存量 GET 的「按字段 LIKE」语义一致（存量按 query 参数逐字段 LIKE）。
	Like map[string]any `json:"like"`
}

// RegisterActions 在路由组 rg 下为 prefix 注册「POST + Action」风格路由（详见《接口规范.md》），
// 复用同一套 handler[T] 业务逻辑。动作集：list / get / create / update / delete /
// batchCreate / batchDelete / status（status 需提供 toggle）。
// Register 已随接口重构下线，此处只保留「POST + Action」单一风格路由。
func RegisterActions[T any](rg *gin.RouterGroup, prefix string, opts Opts[T], toggle *ToggleOpts) *gin.RouterGroup {
	h := &handler[T]{opts: opts}
	g := rg.Group("/" + prefix)
	if opts.V1ListHandler != nil {
		// 自定义列表实现通常沿用「从 query 读分页 / 过滤」的既有逻辑（如 projects / notes），
		// 故先用 envelope.MetaToQuery 把信封 meta 桥接为查询参数，再交给它，保证 v1 语义与存量一致。
		g.POST("/list", func(c *gin.Context) {
			envelope.MetaToQuery(c)
			opts.V1ListHandler(c)
		})
	} else {
		g.POST("/list", h.listAction)
	}
	g.POST("/get", h.getAction)
	if opts.V1CreateHandler != nil {
		// 自定义创建（如笔记的文件优先创建）：绕过基线 createAction 及其 BeforeCreate 校验。
		g.POST("/create", opts.V1CreateHandler)
	} else {
		g.POST("/create", h.createAction)
	}
	g.POST("/update", h.updateAction)
	g.POST("/delete", h.deleteAction)
	g.POST("/batchCreate", h.batchCreateAction)
	g.POST("/batchDelete", h.batchDeleteAction)
	if toggle != nil {
		g.POST("/status", h.statusAction(toggle))
	}
	return g
}

func (h *handler[T]) listAction(c *gin.Context) {
	var meta listMeta
	envelope.GetMeta(c, &meta)
	q := pagination.Query{Page: meta.Page, PageSize: meta.PageSize}
	if q.Page < 1 {
		q.Page = pagination.DefaultPage
	}
	if q.PageSize < 1 || q.PageSize > pagination.MaxPageSize {
		q.PageSize = pagination.DefaultPageSize
	}

	base := config.GetDB().Model(new(T))
	if kw := strings.TrimSpace(meta.Keyword); kw != "" && len(h.opts.Searchable) > 0 {
		ors := make([]string, 0, len(h.opts.Searchable))
		args := make([]any, 0, len(h.opts.Searchable))
		for _, f := range h.opts.Searchable {
			ors = append(ors, f+" LIKE ?")
			args = append(args, "%"+kw+"%")
		}
		base = base.Where(strings.Join(ors, " OR "), args...)
	}
	for _, f := range h.opts.Searchable {
		v, ok := meta.Like[f]
		if !ok {
			continue
		}
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s != "" {
			base = base.Where(f+" LIKE ?", "%"+s+"%")
		}
	}
	for k, v := range meta.Filter {
		if v != nil {
			base = base.Where(k+" = ?", v)
		}
	}
	if sf := meta.Sort; sf != "" && h.sortable(sf) {
		dir := "ASC"
		if strings.EqualFold(meta.Order, "desc") {
			dir = "DESC"
		}
		base = base.Order(sf + " " + dir)
	} else {
		base = base.Order("id DESC")
	}

	var list []*T
	total, err := pagination.Paginate[*T](base, &list, q)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}
	var items any = list
	if h.opts.ToResponse != nil {
		items = h.opts.ToResponse(list)
	}
	renv.Success(c, pagination.New(items, total, q))
}

func (h *handler[T]) getAction(c *gin.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	var row T
	if err := config.GetDB().First(&row, "id = ?", req.ID).Error; err != nil {
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	renv.Success(c, h.viewOne(&row))
}

func (h *handler[T]) createAction(c *gin.Context) {
	ptr := new(T)
	if err := envelope.Bind(c, ptr); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := h.runHook(h.opts.BeforeCreate, ptr); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	tx := config.GetDB().Begin()
	if err := tx.Create(ptr).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	if err := h.runHook(h.opts.AfterCreate, ptr); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	renv.Success(c, h.viewOne(ptr))
}

func (h *handler[T]) updateAction(c *gin.Context) {
	var idReq struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &idReq); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	tx := config.GetDB().Begin()
	var ptr T
	if err := tx.First(&ptr, "id = ?", idReq.ID).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	var patch map[string]any
	if err := envelope.Bind(c, &patch); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	for _, k := range []string{"ID", "id", "CreatedAt", "created_at", "UpdatedAt", "updated_at", "IsDeleted", "is_deleted"} {
		delete(patch, k)
	}
	for key := range patch {
		lk := camelToSnake(key)
		for _, p := range h.opts.Protected {
			if lk == camelToSnake(p) {
				delete(patch, key)
				break
			}
		}
	}
	if err := tx.Model(&ptr).Updates(patch).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	tx.First(&ptr, "id = ?", idReq.ID)
	if err := h.runHook(h.opts.AfterUpdate, &ptr); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	renv.Success(c, h.viewOne(&ptr))
}

func (h *handler[T]) deleteAction(c *gin.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	tx := config.GetDB().Begin()
	var ptr T
	if err := tx.First(&ptr, "id = ?", req.ID).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	if err := tx.Delete(&ptr).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	if err := h.runHook(h.opts.AfterDelete, &ptr); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

func (h *handler[T]) batchCreateAction(c *gin.Context) {
	var req struct {
		Items []T `json:"items"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if len(req.Items) == 0 {
		fail(c, http.StatusBadRequest, "items 为空")
		return
	}
	tx := config.GetDB().Begin()
	for i := range req.Items {
		if err := h.runHook(h.opts.BeforeCreate, &req.Items[i]); err != nil {
			tx.Rollback()
			fail(c, http.StatusBadRequest, err)
			return
		}
	}
	if err := tx.Create(&req.Items).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "批量创建失败: "+err.Error())
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "批量创建失败: "+err.Error())
		return
	}
	var items any = req.Items
	renv.Success(c, items)
}

func (h *handler[T]) batchDeleteAction(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) == 0 {
		fail(c, http.StatusBadRequest, "ids 为空")
		return
	}
	tx := config.GetDB().Begin()
	if err := tx.Where("id IN ?", req.IDs).Delete(new(T)).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "批量删除失败: "+err.Error())
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusInternalServerError, "批量删除失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

func (h *handler[T]) statusAction(toggle *ToggleOpts) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID     int  `json:"id"`
			Status bool `json:"status"`
		}
		if err := envelope.Bind(c, &req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		row, err := toggle.Setter(req.ID, req.Status)
		if err != nil {
			httpx.MapError(c, err, toggle.ErrRules)
			return
		}
		renv.Success(c, row)
	}
}
