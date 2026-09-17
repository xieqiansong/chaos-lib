// Package dbmonitor 提供数据库自省（只读）能力：列举库内用户表、
// 各表行数 / 大小 / 索引等统计信息。兼容 SQLite 与 PostgreSQL 双后端。
//
// 设计要点：
//   - 不新建任何业务表，纯只读自省，无需 AutoMigrate / sql 变更日志；
//   - 统一返回结构 TableStat，后端按 config.Database.Type 分两套实现；
//   - SQLite 无 dbstat 虚拟表时，单表大小降级为 0 + SizeSupported=false。
package dbmonitor

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"chaos-go/config"

	"github.com/gin-gonic/gin"
)

// errTableNotFound 表不存在（detail 接口返回）。
var errTableNotFound = errors.New("table not found")

// TableStat 单表统计（SQLite / PostgreSQL 字段并集）。
type TableStat struct {
	Name          string  `json:"name"`
	Rows          int64   `json:"rows"`
	RowsEstimated bool    `json:"rowsEstimated"` // true=统计估值, false=COUNT(*) 精确
	TableBytes    int64   `json:"tableBytes"`
	IndexBytes    int64   `json:"indexBytes"`
	TotalBytes    int64   `json:"totalBytes"` // 表 + 索引（+ toast）
	IndexCount    int     `json:"indexCount"`
	SizeSupported bool    `json:"sizeSupported"`
	SeqScan       *int64  `json:"seqScan,omitempty"`
	IdxScan       *int64  `json:"idxScan,omitempty"`
	LastVacuum    *string `json:"lastVacuum,omitempty"`
	LastAnalyze   *string `json:"lastAnalyze,omitempty"`
}

// DbOverview 库级总览。
type DbOverview struct {
	DbType        string `json:"dbType"`
	Version       string `json:"version"`
	TotalBytes    int64  `json:"totalBytes"`
	TableCount    int    `json:"tableCount"`
	TotalRows     int64  `json:"totalRows"`
	SizeSupported bool   `json:"sizeSupported"`
}

// ColumnInfo 列定义。
type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	IsPK     bool    `json:"isPk"`
	Default  *string `json:"default,omitempty"`
}

// IndexInfo 索引定义。
type IndexInfo struct {
	Name    string `json:"name"`
	Unique  bool   `json:"unique"`
	Columns string `json:"columns"` // 逗号分隔的列
}

// TableDetail 单表详情（统计 + 列 + 索引）。
type TableDetail struct {
	TableStat
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

var tableNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isSQLite() bool {
	return config.GetConfig().Database.Type == "sqlite"
}

func validTableName(name string) bool {
	return tableNameRe.MatchString(name)
}

// ── handler ──

// GetOverview 返回库级总览（类型 / 版本 / 总大小 / 表数 / 总行数）。
func GetOverview(c *gin.Context) {
	ov, err := getOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ov)
}

// ListTables 返回全部用户表统计（行数为精确 COUNT(*)，支持 ?sort / ?order）。
func ListTables(c *gin.Context) {
	tables, err := collectTables()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sortTables(tables, c.Query("sort"), c.Query("order"))
	c.JSON(http.StatusOK, gin.H{"items": tables, "total": len(tables)})
}

// GetTableDetail 返回单表详情（统计 + 列 + 索引）。
func GetTableDetail(c *gin.Context) {
	name := c.Param("name")
	if !validTableName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的表名"})
		return
	}
	detail, err := getTableDetail(name)
	if errors.Is(err, errTableNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "表不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// ── 分派 ──

func collectTables() ([]TableStat, error) {
	if isSQLite() {
		return collectTablesSQLite()
	}
	return collectTablesPostgres()
}

func getOverview() (*DbOverview, error) {
	if isSQLite() {
		return overviewSQLite()
	}
	return overviewPostgres()
}

func getTableDetail(name string) (*TableDetail, error) {
	if isSQLite() {
		return tableDetailSQLite(name)
	}
	return tableDetailPostgres(name)
}

// ── 排序 ──

func sortTables(tables []TableStat, sortKey, order string) {
	desc := strings.EqualFold(order, "desc")
	less := func(i, j int) bool {
		a, b := tables[i], tables[j]
		var ai, bi int64
		switch sortKey {
		case "rows":
			ai, bi = a.Rows, b.Rows
		case "tableBytes":
			ai, bi = a.TableBytes, b.TableBytes
		case "indexBytes":
			ai, bi = a.IndexBytes, b.IndexBytes
		case "totalBytes":
			ai, bi = a.TotalBytes, b.TotalBytes
		default: // name
			if a.Name != b.Name {
				if desc {
					return a.Name > b.Name
				}
				return a.Name < b.Name
			}
			return false
		}
		if ai == bi {
			return a.Name < b.Name
		}
		if desc {
			return ai > bi
		}
		return ai < bi
	}
	sort.Slice(tables, less)
}
