package dbmonitor

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.Register("db-monitors", Register)
	routehub.RegisterV1("db-monitors", RegisterV1)
}

// Register 把数据库监控（只读自省）的路由挂载到给定路由组（通常来自 routes.go 的 api 组），
// 使本资源的接口自包含、按业务分离：搜索本业务只需看 internal/dbmonitor，搜索本路由只需看这里。
func Register(rg *gin.RouterGroup) {
	g := rg.Group("/db-monitors")
	{
		g.GET("/overview", GetOverview)
		g.GET("/tables", ListTables)
		g.GET("/tables/:name", GetTableDetail)
	}
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 动作：overview（库级总览）、listTables（表分页，参数来自信封 meta）、getTable（单表详情，表名来自 data.name）。
func RegisterV1(rg *gin.RouterGroup) {
	g := rg.Group("/db-monitors")
	g.POST("/overview", GetOverview)
	g.POST("/listTables", listTablesV1)
	g.POST("/getTable", getTableV1)
}

// listTablesV1 是 ListTables 的「POST + Action」版：分页与过滤取自信封 meta。
func listTablesV1(c *gin.Context) {
	var m struct {
		Name     string `json:"name"`
		Sort     string `json:"sort"`
		Order    string `json:"order"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	envelope.GetMeta(c, &m)
	q := pagination.Query{Page: m.Page, PageSize: m.PageSize}
	if q.Page < 1 {
		q.Page = pagination.DefaultPage
	}
	if q.PageSize < 1 || q.PageSize > pagination.MaxPageSize {
		q.PageSize = pagination.DefaultPageSize
	}
	items, total, err := ListTableStats(m.Name, m.Sort, m.Order, q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, pagination.New(items, total, q))
}

// getTableV1 是 GetTableDetail 的「POST + Action」版：表名取自信封 data.name。
func getTableV1(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := envelope.Bind(c, &req); err != nil || req.Name == "" {
		renv.Error(c, http.StatusBadRequest, "表名不能为空")
		return
	}
	detail, err := TableDetailOf(req.Name)
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
