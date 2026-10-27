package dbmonitor

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/pagination"
	"chaos-go/internal/framework/plugin"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

func init() {
	plugin.Register(plugin.Registration{
		Meta: plugin.Meta{
			ID:           "dbmonitor",
			Name:         "数据监控",
			Version:      "1.0.0",
			Dependencies: nil,
		},
		RegisterRoutes: func(rg *web.RouterGroup) {
			RegisterV1(rg)
		},
	})
}

func RegisterV1(rg *web.RouterGroup) {
	g := rg.Group("/db-monitors")
	g.POST("/overview", GetOverview)
	g.POST("/listTables", listTablesV1)
	g.POST("/getTable", getTableV1)
}

func listTablesV1(c *web.Context) {
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

func getTableV1(c *web.Context) {
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

func GetOverview(c *web.Context) {
	ov, err := Overview()
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, ov)
}

func ListTables(c *web.Context) {
	q := pagination.Parse(c)
	items, total, err := ListTableStats(c.Query("name"), c.Query("sort"), c.Query("order"), q)
	if err != nil {
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, pagination.New(items, total, q))
}

func GetTableDetail(c *web.Context) {
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
