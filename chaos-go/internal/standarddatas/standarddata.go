// Package standarddatas 是「标准参考表」：覆盖常见数据类型 + 统一基字段
// （主键 / 创建时间 / 更新时间 / 逻辑删除），作为所有简单表的基准范式。
//
// 本包只声明模型与 Register（把路由交给通用 crud 处理），不写任何 handler，
// 以此演示「资源接口自包含、按业务分离」的目标：搜索本业务只需看本目录。
package standarddatas

import (
	"time"

	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
)

// StandardData 标准数据行。
// 嵌入 crud.BaseModel 自动获得 ID / CreatedAt / UpdatedAt / IsDeleted。
type StandardData struct {
	crud.BaseModel
	Name        string     `json:"Name"`
	Code        string     `json:"Code"`
	Description string     `json:"Description"`
	Category    string     `json:"Category"`
	Quantity    int        `json:"Quantity"`
	Price       float64    `json:"Price"`
	Enabled     bool       `json:"Enabled"`
	Config      string     `json:"Config"` // 扩展 JSON（以文本存储，展示 json 类型列）
	EffectiveAt *time.Time `json:"EffectiveAt"`
	Sort        int        `json:"Sort" gorm:"default:0"`
}

// TableName 显式指定表名（与前端资源名 standardDatas 对应）。
func (StandardData) TableName() string { return "standard_datas" }

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 搜索/排序白名单、状态子路由等均在 Opts 中声明，无需额外 handler。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "standardDatas", &StandardData{}, crud.Opts{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
		HasStatus:  true,
	})
}
