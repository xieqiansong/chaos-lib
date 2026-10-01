package dbmonitor

import (
	"errors"
	"net/http"
	"strings"

	"chaos-go/internal/pagination"
	renv "chaos-go/internal/resp"

	"github.com/gin-gonic/gin"
)

// GetOverview 返回库级总览（类型 / 版本 / 总大小 / 表数 / 总行数）。
func GetOverview(c *gin.Context) {
	ov, err := getOverview()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, ov)
}

// ListTables 返回用户表统计的分页列表（行数为精确 COUNT(*)，
// 支持 ?page / ?page_size / ?sort / ?order / ?name）。分页与排序约定与标准
// CRUD 基线（pagination 包）一致：排序键为 snake_case，响应为
// { list, pagination: { page, page_size, total, total_pages } }。
func ListTables(c *gin.Context) {
	all, err := collectTables()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 按表名过滤（在分页与排序前生效，并计入 total）。
	name := strings.TrimSpace(c.Query("name"))
	filtered := all
	if name != "" {
		lower := strings.ToLower(name)
		filtered = make([]TableStat, 0, len(all))
		for _, t := range all {
			if strings.Contains(strings.ToLower(t.Name), lower) {
				filtered = append(filtered, t)
			}
		}
	}

	sortTables(filtered, c.Query("sort"), c.Query("order"))

	// 分页（复用统一约定：page 默认 1，page_size 默认 20，上限 200）。
	q := pagination.Parse(c)
	total := int64(len(filtered))
	start := q.Offset()
	if start > int(total) {
		start = int(total)
	}
	end := start + q.PageSize
	if end > int(total) {
		end = int(total)
	}
	items := filtered[start:end]
	if items == nil {
		items = []TableStat{}
	}
	renv.Success(c, pagination.New(items, total, q))
}

// GetTableDetail 返回单表详情（统计 + 列 + 索引）。
func GetTableDetail(c *gin.Context) {
	name := c.Param("name")
	if !validTableName(name) {
		renv.Error(c, http.StatusBadRequest, "非法的表名")
		return
	}
	detail, err := getTableDetail(name)
	if errors.Is(err, errTableNotFound) {
		renv.Error(c, http.StatusNotFound, "表不存在")
		return
	}
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, detail)
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
