// Package standarddata 是「标准参考表」：覆盖常见数据类型 + 统一基字段
// （主键 / 创建时间 / 更新时间 / 逻辑删除），作为所有简单表的基准范式。
//
// 本包的 Register 把纯 CRUD 交给通用 crud，状态切换这类「扩展能力」则在基线之外
// 由本业务包自实现并挂载为自定义路由。以此演示「资源接口自包含、按业务分离」：
// 搜索本业务只需看本目录，且标准 CRUD 不被业务特例污染。
package standarddata

import (
	"net/http"
	"time"

	"chaos-go/config"
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

// TableName 显式指定表名（与前端资源名 standardData 对应）。
func (StandardData) TableName() string { return "standard_datas" }

// Register 把本资源的路由挂载到给定路由组（通常来自 routes.go 的 api 组）。
// 纯 CRUD 交给通用 crud；状态切换在基线之外由本包自实现并挂载，避免污染标准实现。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "standardData", &StandardData{}, crud.Opts{
		Searchable: []string{"name", "code", "description"},
		Sortable:   []string{"id", "sort", "created_at"},
	})
	// 自定义子路由：状态切换（标准 CRUD 之外的本业务实现）
	rg.Group("/standardData").PATCH("/:id/status", status)
}

// status 状态切换（本业务包的自定义子路由实现：PATCH /standardData/:id/status，body {status:bool}）。
func status(c *gin.Context) {
	var req struct {
		Status bool `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var row StandardData
	if err := config.GetDB().Where("is_deleted = ?", false).First(&row, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if err := config.GetDB().Model(&row).Update("enabled", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "状态更新失败: " + err.Error()})
		return
	}
	config.GetDB().First(&row, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "状态更新成功", "data": &row})
}
