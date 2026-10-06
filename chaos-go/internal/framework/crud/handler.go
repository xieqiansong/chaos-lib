package crud

import (
	"net/http"
	"strings"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// Register[T] 在路由组 rg 下为 prefix 注册一套标准 CRUD 路由，返回该资源的路由组。
// 模型类型由类型参数 T 推导（无需再传零值指针），同时约束了回调与切片的元素类型。
// 返回值供业务包直接挂载扩展子路由（避免前缀字符串二次书写），状态切换用 RegisterToggle。
func Register[T any](rg *gin.RouterGroup, prefix string, opts Opts[T]) *gin.RouterGroup {
	h := &handler[T]{opts: opts}
	g := rg.Group("/" + prefix)
	if opts.ListHandler != nil {
		g.GET("", opts.ListHandler)
	} else {
		g.GET("", h.list)
	}
	g.GET("/:id", h.get)
	g.POST("", h.create)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	return g
}

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

// list 列表：分页 + 搜索 + 排序 + 软删过滤。
func (h *handler[T]) list(c *gin.Context) {
	q := pagination.Parse(c)
	// 软删过滤由 soft_delete 插件自动追加，此处不再手写 is_deleted 条件
	base := config.GetDB().Model(new(T))

	for _, f := range h.opts.Searchable {
		if v := strings.TrimSpace(c.Query(f)); v != "" {
			base = base.Where(f+" LIKE ?", "%"+v+"%")
		}
	}

	if sf := c.Query("sort"); sf != "" && h.sortable(sf) {
		dir := "ASC"
		if c.Query("order") == "desc" {
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
	// ToResponse 未配置时直接返回原始切片，避免无谓转换。
	var items any = list
	if h.opts.ToResponse != nil {
		items = h.opts.ToResponse(list)
	}
	renv.Success(c, pagination.New(items, total, q))
}

// get 单条（含软删过滤）。
func (h *handler[T]) get(c *gin.Context) {
	var row T
	if err := config.GetDB().First(&row, "id = ?", c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	renv.Success(c, h.viewOne(&row))
}

// create 创建（事务内执行 AfterCreate，钩子失败即回滚）。
func (h *handler[T]) create(c *gin.Context) {
	ptr := new(T)
	if err := c.ShouldBindJSON(ptr); err != nil {
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
		fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	renv.Success(c, h.viewOne(ptr))
}

// update 部分字段更新（PATCH，事务内执行 AfterUpdate，钩子失败即回滚）。
func (h *handler[T]) update(c *gin.Context) {
	tx := config.GetDB().Begin()
	var ptr T
	if err := tx.First(&ptr, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	// 基字段不可经 PATCH 直接改写
	for _, k := range []string{"ID", "id", "CreatedAt", "created_at", "UpdatedAt", "updated_at", "IsDeleted", "is_deleted"} {
		delete(patch, k)
	}
	// 受保护字段只能走带副作用的专属路由。
	// 前端透传的是驼峰字段名（如 Status），而 Opts 通常写列名（status），故统一转 snake 后比较。
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
	tx.First(&ptr, "id = ?", c.Param("id"))
	if err := h.runHook(h.opts.AfterUpdate, &ptr); err != nil {
		tx.Rollback()
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	renv.Success(c, h.viewOne(&ptr))
}

// delete 软删除（事务内执行 AfterDelete，钩子失败即回滚）。
func (h *handler[T]) delete(c *gin.Context) {
	tx := config.GetDB().Begin()
	var ptr T
	if err := tx.First(&ptr, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	// Delete 在 soft_delete 插件下即软删：插件把语句改写为 UPDATE ... SET is_deleted = 1
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
		fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	renv.Success(c, nil)
}

// ---- 「POST + Action」风格（v1） ----

// listMeta 信封 meta 中的列表查询参数（对应规范 list 动作）。
type listMeta struct {
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Sort     string         `json:"sort"`
	Order    string         `json:"order"`
	Filter   map[string]any `json:"filter"`
	Keyword  string         `json:"keyword"`
}

// RegisterActions 在路由组 rg 下为 prefix 注册「POST + Action」风格路由（详见《接口规范.md》），
// 复用同一套 handler[T] 业务逻辑。动作集：list / get / create / update / delete /
// batchCreate / batchDelete / status（status 需提供 toggle）。
// 与 Register 的 RESTful 路由并存，用于接口重构的双轨迁移。
func RegisterActions[T any](rg *gin.RouterGroup, prefix string, opts Opts[T], toggle *ToggleOpts) *gin.RouterGroup {
	h := &handler[T]{opts: opts}
	g := rg.Group("/" + prefix)
	g.POST("/list", h.listAction)
	g.POST("/get", h.getAction)
	g.POST("/create", h.createAction)
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
