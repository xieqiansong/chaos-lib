// Package crud 提供「配置即接口」的通用 REST 处理能力，作为所有简单表的标准基线。
//
// 设计目标：对一个只含「列表/详情/创建/更新/删除(软删)」的普通表，
// 只需在业务包里定义模型并调用 crud.Register[Model]，无需手写任何 handler。
// 业务特有的派生字段与副作用通过 Opts[T] 的回调（ToResponse / AfterXxx）注入，基线不感知具体业务。
//
// 约定（与前端 useRestApi 严格对应）：
//   - 列表 GET  /<prefix>           query: page, size, sort, order, <可搜字段>
//   - 详情 GET  /<prefix>/:id
//   - 创建 POST /<prefix>           body: 创建字段
//   - 更新 PATCH /<prefix>/:id      body: 部分字段
//   - 删除 DELETE /<prefix>/:id     软删除（is_deleted = true）
//
// 状态切换等扩展能力不内置在基线里，而是由业务包按需以「自定义路由」自行实现并挂载
// （见各业务包的 Register）。这样基线只负责纯 CRUD，扩展能力下沉到业务包，互不污染。
//
// 列表响应统一为分页结构 { items, total, page, size }；单条/创建/更新返回
// { message, data }；删除返回 { message }；错误返回 { error }。
package crud

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"chaos-go/config"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
)

// BaseModel 标准基字段：所有简单表嵌入它即可获得统一的
// 主键 / 创建时间 / 更新时间 / 逻辑删除。IsDeleted 不对外暴露（json:"-"）。
type BaseModel struct {
	ID        int       `gorm:"primaryKey" json:"ID"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	IsDeleted bool      `gorm:"default:false" json:"-"`
}

// Opts[T] 资源级配置。T 为业务模型类型（如 StandardData），约束了回调与切片的元素类型。
type Opts[T any] struct {
	// Searchable: 参与模糊搜索的「列名」（snake_case），前端按字段名自动转 snake 后透传。
	Searchable []string
	// Sortable: 允许排序的「列名」白名单（snake_case），未列出则忽略 sort 参数。
	Sortable []string
	// Protected: PATCH 更新时剔除的业务字段（如 status）。
	// 用于「只能通过带副作用的专属路由改写」的字段，避免通用 PATCH 绕过副作用。
	Protected []string

	// ToResponse: 读方向回调，把 []*T 整批转换为响应形态（通常 DTO 切片）。
	// 未配置则原样返回模型；单条接口（get/create/update）内部包成 1 元素切片复用同一回调并取首元素。
	ToResponse func(rows []*T) any

	// BeforeCreate: 写方向回调，在 ShouldBindJSON 之后、tx.Create 之前执行。
	// 入参为 *T，可就地规范化字段；返回 error 则直接 400 并拒绝创建。
	BeforeCreate func(row *T) error

	// AfterCreate / AfterUpdate / AfterDelete: 写方向回调，在事务内、提交前执行。
	// 入参为 *T；返回 error 则整笔回滚（钩子不成功就不提交）。
	AfterCreate func(row *T) error
	AfterUpdate func(row *T) error
	AfterDelete func(row *T) error

	// ListHandler: 可选列表处理器覆盖。设置后，GET /<prefix> 走该自定义实现而非基线 list，
	// 用于列表需要「派生数据 / 外部副作用（如扫描磁盘）」的场景（如项目管理：合并已认领 + 未认领目录）。
	// 自定义实现须自行处理分页/搜索/软删过滤，并返回统一分页结构 { items, total, page, size }。
	ListHandler func(c *gin.Context)
}

// Register[T] 在路由组 rg 下为 prefix 注册一套标准 CRUD 路由。
// 模型类型由类型参数 T 推导（无需再传零值指针），同时约束了回调与切片的元素类型。
// 状态切换等扩展能力不在基线内，由业务包自行实现并挂载（见各业务包的 Register）。
func Register[T any](rg *gin.RouterGroup, prefix string, opts Opts[T]) {
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

// firstElem 从 ToResponse 返回的切片中取出首元素（单条接口复用批量回调）。
// 是基线上唯一保留的反射点：ToResponse 的返回类型对包不可知，只能运行时取首元素。
func firstElem(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		return rv.Index(0).Interface()
	}
	return v
}

// fail 统一错误响应：msg 可为 error 或任意值（字符串/拼接串），集中维护错误信封形态。
func fail(c *gin.Context, status int, msg any) {
	if e, ok := msg.(error); ok {
		c.JSON(status, gin.H{"error": e.Error()})
		return
	}
	c.JSON(status, gin.H{"error": fmt.Sprintf("%v", msg)})
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
	c.JSON(http.StatusOK, pagination.New(items, total, q))
}

// get 单条（含软删过滤）。
func (h *handler[T]) get(c *gin.Context) {
	var row T
	if err := config.GetDB().Where("is_deleted = ?", false).First(&row, "id = ?", c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "记录不存在")
		return
	}
	c.JSON(http.StatusOK, h.viewOne(&row))
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
	c.JSON(http.StatusOK, gin.H{"message": "创建成功", "data": h.viewOne(ptr)})
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
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "data": h.viewOne(&ptr)})
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
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// camelToSnake 把 Status 这类驼峰字段名转成 status，便于 Protected 同时覆盖两种写法。
func camelToSnake(s string) string {
	out := make([]rune, 0, len(s)+4)
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		out = append(out, r)
	}
	return strings.ToLower(string(out))
}
