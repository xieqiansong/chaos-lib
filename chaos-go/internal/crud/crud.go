// Package crud 提供「配置即接口」的通用 REST 处理能力，作为所有简单表的标准基线。
//
// 设计目标：对一个只含「列表/详情/创建/更新/删除(软删)」的普通表，
// 只需在业务包里定义模型并调用 crud.Register，无需手写任何 handler。
// 业务特有的派生字段与副作用通过 Opts 的回调（ToResponse / AfterXxx）注入，基线不感知具体业务。
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
	"net/http"
	"reflect"
	"strings"
	"time"

	"chaos-go/config"
	"chaos-go/internal/pagination"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BaseModel 标准基字段：所有简单表嵌入它即可获得统一的
// 主键 / 创建时间 / 更新时间 / 逻辑删除。IsDeleted 不对外暴露（json:"-"）。
type BaseModel struct {
	ID        int       `gorm:"primaryKey" json:"ID"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	IsDeleted bool      `gorm:"default:false" json:"-"`
}

// Opts 资源级配置。
type Opts struct {
	// Searchable: 参与模糊搜索的「列名」（snake_case），前端按字段名自动转 snake 后透传。
	Searchable []string
	// Sortable: 允许排序的「列名」白名单（snake_case），未列出则忽略 sort 参数。
	Sortable []string
	// Protected: PATCH 更新时剔除的业务字段（如 status）。
	// 用于「只能通过带副作用的专属路由改写」的字段，避免通用 PATCH 绕过副作用。
	Protected []string

	// ToResponse: 读方向回调，把查询结果整批转换为响应形态。
	// 入参固定为 []*Model（整批而非逐行，便于业务侧批量/并发优化），返回用于 JSON 序列化的切片。
	// 未配置则原样返回模型。单条接口（get/create/update）内部包成 1 元素切片复用同一回调。
	ToResponse func(rows any) any

	// AfterCreate / AfterUpdate / AfterDelete: 写方向回调，在事务内、提交前执行。
	// 入参为 *Model；返回 error 则整笔回滚（钩子不成功就不提交）。
	// 注意：文件系统等外部副作用本身无法随事务回滚，钩子放在提交前只是保证「失败即不落库」。
	AfterCreate func(row any) error
	AfterUpdate func(row any) error
	AfterDelete func(row any) error
}

// Register 在路由组 rg 下为 prefix 注册一套标准 CRUD 路由。
// model 传入零值指针（如 &StandardData{}），用于反射推导表结构与类型。
// 状态切换等扩展能力不在基线内，由业务包自行实现并挂载（见各业务包的 Register）。
func Register(rg *gin.RouterGroup, prefix string, model any, opts Opts) {
	h := &handler{model: model, opts: opts}
	g := rg.Group("/" + prefix)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.POST("", h.create)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
}

// handler 持有模型类型信息与资源选项，按方法分发。
type handler struct {
	model any
	opts  Opts
}

func (h *handler) elemType() reflect.Type { return reflect.TypeOf(h.model).Elem() }

// newModel 返回指向新实例的指针（如 *StandardData）。
func (h *handler) newModel() any { return reflect.New(h.elemType()).Interface() }

// newSlice 返回指向新切片（如 *[]StandardData）的指针。
func (h *handler) newSlice() any {
	return reflect.New(reflect.SliceOf(h.elemType())).Interface()
}

func (h *handler) sortable(field string) bool {
	for _, f := range h.opts.Sortable {
		if f == field {
			return true
		}
	}
	return false
}

// toPtrSlice 把 Find 出来的 []Model 转成 []*Model，统一 ToResponse 的入参类型。
func (h *handler) toPtrSlice(slice any) any {
	v := reflect.ValueOf(slice)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	ptrs := reflect.MakeSlice(reflect.SliceOf(reflect.PtrTo(h.elemType())), 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		ptrs = reflect.Append(ptrs, v.Index(i).Addr())
	}
	return ptrs.Interface()
}

// viewRows 整批交给 ToResponse；未配置则原样返回。
func (h *handler) viewRows(rows any) any {
	if h.opts.ToResponse == nil {
		return rows
	}
	return h.opts.ToResponse(rows)
}

// viewOne 单条：包成 1 元素切片复用同一个批量回调，再取首元素。
func (h *handler) viewOne(row any) any {
	if h.opts.ToResponse == nil {
		return row
	}
	sv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(row)), 1, 1)
	sv.Index(0).Set(reflect.ValueOf(row))
	out := h.opts.ToResponse(sv.Interface())
	if out == nil {
		return row
	}
	ov := reflect.ValueOf(out)
	if ov.Kind() == reflect.Slice && ov.Len() > 0 {
		return ov.Index(0).Interface()
	}
	return row
}

// runHook 执行写方向回调；返回 error 时调用方负责回滚并响应。
func (h *handler) runHook(hook func(row any) error, row any) error {
	if hook == nil {
		return nil
	}
	return hook(row)
}

// list 列表：分页 + 搜索 + 排序 + 软删过滤。
func (h *handler) list(c *gin.Context) {
	q := pagination.Parse(c)
	base := config.GetDB().Model(h.newModel()).Where("is_deleted = ?", false)

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

	slice := h.newSlice()
	var total int64
	if err := base.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	// Session 克隆避免 Count 的 SELECT 子句污染后续 Find。
	if err := base.Session(&gorm.Session{}).Scopes(q.Scope).Find(slice).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, pagination.New(h.viewRows(h.toPtrSlice(slice)), total, q))
}

// get 单条（含软删过滤）。
func (h *handler) get(c *gin.Context) {
	ptr := h.newModel()
	if err := config.GetDB().Where("is_deleted = ?", false).First(ptr, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	c.JSON(http.StatusOK, h.viewOne(ptr))
}

// create 创建（事务内执行 AfterCreate，钩子失败即回滚）。
func (h *handler) create(c *gin.Context) {
	ptr := h.newModel()
	if err := c.ShouldBindJSON(ptr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tx := config.GetDB().Begin()
	if err := tx.Create(ptr).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	if err := h.runHook(h.opts.AfterCreate, ptr); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建成功", "data": h.viewOne(ptr)})
}

// update 部分字段更新（PATCH，事务内执行 AfterUpdate，钩子失败即回滚）。
func (h *handler) update(c *gin.Context) {
	tx := config.GetDB().Begin()
	ptr := h.newModel()
	if err := tx.Where("is_deleted = ?", false).First(ptr, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	if err := tx.Model(ptr).Updates(patch).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	tx.First(ptr, "id = ?", c.Param("id"))
	if err := h.runHook(h.opts.AfterUpdate, ptr); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "data": h.viewOne(ptr)})
}

// delete 软删除（事务内执行 AfterDelete，钩子失败即回滚）。
func (h *handler) delete(c *gin.Context) {
	tx := config.GetDB().Begin()
	ptr := h.newModel()
	if err := tx.Where("is_deleted = ?", false).First(ptr, "id = ?", c.Param("id")).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err := tx.Model(ptr).Update("is_deleted", true).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	if err := h.runHook(h.opts.AfterDelete, ptr); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
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
