// Package pagination 提供统一的分页解析与查询封装，供所有列表接口复用。
//
// 约定：
//   - 通过 query 参数 page（默认 1）、size（默认 20）传参；
//   - size 强制约束在 [1, MaxSize] 区间；
//   - 响应统一为 { items, total, page, size }。
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultPage = 1
	DefaultSize = 20
	MaxSize     = 200
)

// Query 归一化后的分页参数。
type Query struct {
	Page int
	Size int
}

// Parse 从 gin 上下文读取 page/size，套用默认值与边界约束。
func Parse(c *gin.Context) Query {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = DefaultPage
	}
	if size < 1 || size > MaxSize {
		size = DefaultSize
	}
	return Query{Page: page, Size: size}
}

// Offset 返回当前页对应的 SQL 偏移量。
func (q Query) Offset() int {
	return (q.Page - 1) * q.Size
}

// Scope 作为 gorm Scopes 使用，追加 Limit/Offset。
func (q Query) Scope(db *gorm.DB) *gorm.DB {
	return db.Offset(q.Offset()).Limit(q.Size)
}

// Result 标准分页响应结构。
type Result struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// New 构造标准分页响应。
func New(items interface{}, total int64, q Query) Result {
	return Result{Items: items, Total: total, Page: q.Page, Size: q.Size}
}

// Paginate 在已带 Where/Order 的查询 base 上执行 count + 分页查询。
// base 不应自带 Limit/Offset；返回总条数与错误。
func Paginate[T any](base *gorm.DB, dest *[]T, q Query) (int64, error) {
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return 0, err
	}
	// Session 克隆避免 Count 的 SELECT 子句污染后续 Find。
	if err := base.Session(&gorm.Session{}).Scopes(q.Scope).Find(dest).Error; err != nil {
		return 0, err
	}
	return total, nil
}
