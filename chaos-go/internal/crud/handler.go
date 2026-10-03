package crud

import (
	"net/http"
	"strings"

	"chaos-go/internal/config"
	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"

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
	base := config.GetDB().Model(new(T)).Where("is_deleted = ?", false)

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
	if err := config.GetDB().Where("is_deleted = ?", false).First(&row, "id = ?", c.Param("id")).Error; err != nil {
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
	if err := tx.Where("is_deleted = ?", false).First(&ptr, "id = ?", c.Param("id")).Error; err != nil {
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
	if err := tx.Where("is_deleted = ?", false).First(&ptr, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	if err := tx.Model(&ptr).Update("is_deleted", true).Error; err != nil {
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
