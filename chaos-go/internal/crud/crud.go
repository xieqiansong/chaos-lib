// Package crud 提供「配置即接口」的通用 REST 处理能力，作为所有简单表的标准基线。
//
// 设计目标：对一个只含「列表/详情/创建/更新/删除(软删)/状态切换」的普通表，
// 只需在业务包里定义模型并调用 crud.Register，无需手写任何 handler。
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
	c.JSON(http.StatusOK, pagination.New(reflect.ValueOf(slice).Elem().Interface(), total, q))
}

// get 单条（含软删过滤）。
func (h *handler) get(c *gin.Context) {
	ptr := h.newModel()
	if err := config.GetDB().Where("is_deleted = ?", false).First(ptr, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	c.JSON(http.StatusOK, ptr)
}

// create 创建。
func (h *handler) create(c *gin.Context) {
	ptr := h.newModel()
	if err := c.ShouldBindJSON(ptr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.GetDB().Create(ptr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建成功", "data": ptr})
}

// update 部分字段更新（PATCH）。
func (h *handler) update(c *gin.Context) {
	ptr := h.newModel()
	if err := config.GetDB().First(ptr, "id = ? AND is_deleted = ?", c.Param("id"), false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 基字段不可经 PATCH 直接改写
	for _, k := range []string{"ID", "id", "CreatedAt", "created_at", "UpdatedAt", "updated_at", "IsDeleted", "is_deleted"} {
		delete(patch, k)
	}
	if err := config.GetDB().Model(ptr).Updates(patch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	config.GetDB().First(ptr, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "data": ptr})
}

// delete 软删除。
func (h *handler) delete(c *gin.Context) {
	ptr := h.newModel()
	if err := config.GetDB().First(ptr, "id = ? AND is_deleted = ?", c.Param("id"), false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err := 	config.GetDB().Model(ptr).Update("is_deleted", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
