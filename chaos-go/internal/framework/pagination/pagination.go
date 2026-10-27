package pagination

import (
	"strconv"

	"chaos-go/internal/framework/web"
	"gorm.io/gorm"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
)

type Query struct {
	Page     int
	PageSize int
}

func Parse(c *web.Context) Query {
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

func (q Query) Offset() int {
	return (q.Page - 1) * q.PageSize
}

func (q Query) Scope(db *gorm.DB) *gorm.DB {
	return db.Offset(q.Offset()).Limit(q.PageSize)
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type PageData struct {
	List       interface{} `json:"list"`
	Pagination Pagination  `json:"pagination"`
}

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

	if err := base.Session(&gorm.Session{}).Scopes(q.Scope).Find(dest).Error; err != nil {
		return 0, err
	}
	return total, nil
}
