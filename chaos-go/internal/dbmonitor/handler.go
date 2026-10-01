package dbmonitor

import (
	"errors"
	"net/http"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("dbMonitor", Register)
}

// Register 把数据库监控（只读自省）的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 使本资源的接口自包含、按业务分离：搜索本业务只需看 internal/dbmonitor，搜索本路由只需看这里。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/dbMonitor")
	{
		g.GET("/overview", GetOverview)
		g.GET("/tables", ListTables)
		g.GET("/tables/:name", GetTableDetail)
	}
}

// GetOverview 返回库级总览（类型 / 版本 / 总大小 / 表数 / 总行数）。
func GetOverview(c *gin.Context) {
	ov, err := Overview()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, ov)
}

// ListTables 返回用户表统计的分页列表（?page / ?page_size / ?sort / ?order / ?name）。
// 分页与排序约定与标准 CRUD 基线（pagination 包）一致：排序键为 snake_case。
func ListTables(c *gin.Context) {
	q := pagination.Parse(c)
	items, total, err := ListTableStats(c.Query("name"), c.Query("sort"), c.Query("order"), q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, pagination.New(items, total, q))
}

// GetTableDetail 返回单表详情（统计 + 列 + 索引）。
func GetTableDetail(c *gin.Context) {
	detail, err := TableDetailOf(c.Param("name"))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidTableName):
			renv.Error(c, http.StatusBadRequest, "非法的表名")
		case errors.Is(err, ErrTableNotFound):
			renv.Error(c, http.StatusNotFound, "表不存在")
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	renv.Success(c, detail)
}
