package crud

import (
	"time"

	"chaos-go/internal/framework/web"
	"gorm.io/plugin/soft_delete"
)

type BaseModel struct {
	ID        int                   `gorm:"primaryKey" json:"ID"`
	CreatedAt time.Time             `json:"CreatedAt"`
	UpdatedAt time.Time             `json:"UpdatedAt"`
	IsDeleted soft_delete.DeletedAt `gorm:"softDelete:flag" json:"-"`
}

func (m BaseModel) Deleted() bool {
	return m.IsDeleted != 0
}

type Opts[T any] struct {
	Searchable []string

	Sortable []string

	Protected []string

	ToResponse func(rows []*T) any

	BeforeCreate func(row *T) error

	AfterCreate func(row *T) error
	AfterUpdate func(row *T) error
	AfterDelete func(row *T) error

	V1ListHandler func(c *web.Context)

	V1CreateHandler func(c *web.Context)
}
