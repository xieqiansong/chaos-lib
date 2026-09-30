// Package pagination 提供统一的分页解析与查询封装，供所有列表接口复用。
//
// 入参（query）：page（默认 1）、page_size（默认 20，强制约束 [1, MaxPageSize]）。
// 响应载荷（即统一信封的 data 字段）统一为：
//
//	{
//	  "list": [ ... ],
//	  "pagination": {
//	    "page": 1, "page_size": 20, "total": 123, "total_pages": 7
//	  }
//	}
//
// 外层再套 resp 信封 { code, message, data }（分页接口 data 即 PageData）。
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
)

// Query 归一化后的分页参数。
type Query struct {
	Page     int
	PageSize int
}

// Parse 从 gin 上下文读取 page/page_size，套用默认值与边界约束。
func Parse(c *gin.Context) Query {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}
	return Query{Page: page, PageSize: pageSize}
}

// Offset 返回当前页对应的 SQL 偏移量。
func (q Query) Offset() int {
	return (q.Page - 1) * q.PageSize
}

// Scope 作为 gorm Scopes 使用，追加 Limit/Offset。
func (q Query) Scope(db *gorm.DB) *gorm.DB {
	return db.Offset(q.Offset()).Limit(q.PageSize)
}

// Pagination 分页元信息（信封 data.pagination）。
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// PageData 标准分页响应载荷（即统一信封的 data）。
type PageData struct {
	List       interface{} `json:"list"`
	Pagination Pagination  `json:"pagination"`
}

// New 构造标准分页响应载荷，自动计算 total_pages。
func New(items interface{}, total int64, q Query) PageData {
	totalPages := 0
	if q.PageSize > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return PageData{
		List: items,
		Pagination: Pagination{
			Page:       q.Page,
			PageSize:   q.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// Paginate 在已带 Where/Order 的查询 base 上执行 count + 分页查询。
// base 不应自带 Limit/Offset；返回总条数与错误。
//
// count 依赖 Statement.Model 推导表名，而调用方传入的 base 通常只有
// Where/Order，因此这里用 dest 的元素类型补上模型，否则 GORM 会把
// &total 当成模型解析并报 unsupported data type / Table not set。
func Paginate[T any](base *gorm.DB, dest *[]T, q Query) (int64, error) {
	var total int64
	countTx := base
	if countTx.Statement.Model == nil {
		var model T
		countTx = countTx.Model(&model)
	}
	if err := countTx.Count(&total).Error; err != nil {
		return 0, err
	}
	// Session 克隆避免 Count 的 SELECT 子句污染后续 Find。
	if err := base.Session(&gorm.Session{}).Scopes(q.Scope).Find(dest).Error; err != nil {
		return 0, err
	}
	return total, nil
}
