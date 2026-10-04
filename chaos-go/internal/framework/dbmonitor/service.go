package dbmonitor

import (
	"errors"
	"sort"
	"strings"

	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/pagination"
)

func isSQLite() bool {
	return config.GetConfig().Database.Type == "sqlite"
}

// validTableName 仅做基本准入校验：拒绝空名与会破坏路由分段 /
// 解析的控制字符。真正的 SQL 注入防护由调用方统一使用 %q 标识符
// 引号转义（SQLite 双引号标识符，内部双引号再翻倍）来兜底，因此这里
// 不应再用严格正则误伤含连字符、空格、点、Unicode 的合法表名。
func validTableName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '/' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
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
	var (
		detail *TableDetail
		err    error
	)
	if isSQLite() {
		detail, err = tableDetailSQLite(name)
	} else {
		detail, err = tableDetailPostgres(name)
	}
	if err != nil {
		return nil, err
	}
	// nil slice 会被编码成 JSON null，前端按数组消费会崩；统一归一化为空数组。
	if detail.Columns == nil {
		detail.Columns = []ColumnInfo{}
	}
	if detail.Indexes == nil {
		detail.Indexes = []IndexInfo{}
	}
	return detail, nil
}

// ── 用例 ────────────────────────────────────────────────────────

// Overview 返回库级总览（类型 / 版本 / 总大小 / 表数 / 总行数）。
func Overview() (*DbOverview, error) {
	return getOverview()
}

// TableDetailOf 返回单表详情（统计 + 列 + 索引）。
func TableDetailOf(name string) (*TableDetail, error) {
	if !validTableName(name) {
		return nil, ErrInvalidTableName
	}
	detail, err := getTableDetail(name)
	if err != nil {
		if errors.Is(err, errTableNotFound) {
			return nil, ErrTableNotFound
		}
		return nil, err
	}
	return detail, nil
}

// ListTableStats 返回表统计的分页列表（行数为精确 COUNT(*)）：
// 按表名过滤（在排序与分页前生效，并计入 total），再按 sort / order 排序后分页。
func ListTableStats(name, sortKey, order string, q pagination.Query) ([]TableStat, int64, error) {
	all, err := collectTables()
	if err != nil {
		return nil, 0, err
	}

	filtered := filterTables(all, name)
	sortTables(filtered, sortKey, order)

	total := int64(len(filtered))
	start := q.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + q.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	items := filtered[start:end]
	if items == nil {
		items = []TableStat{}
	}
	return items, total, nil
}

// ── 过滤与排序 ──────────────────────────────────────────────────

// filterTables 按表名子串过滤（大小写不敏感）；name 为空时原样返回。
func filterTables(tables []TableStat, name string) []TableStat {
	name = strings.TrimSpace(name)
	if name == "" {
		return tables
	}
	lower := strings.ToLower(name)
	filtered := make([]TableStat, 0, len(tables))
	for _, t := range tables {
		if strings.Contains(strings.ToLower(t.Name), lower) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// sortTables 按给定键与方向排序；未识别的键按表名排。
func sortTables(tables []TableStat, sortKey, order string) {
	desc := strings.EqualFold(order, "desc")
	less := func(i, j int) bool {
		a, b := tables[i], tables[j]
		var ai, bi int64
		switch sortKey {
		case "rows":
			ai, bi = a.Rows, b.Rows
		case "table_bytes":
			ai, bi = a.TableBytes, b.TableBytes
		case "index_bytes":
			ai, bi = a.IndexBytes, b.IndexBytes
		case "total_bytes":
			ai, bi = a.TotalBytes, b.TotalBytes
		case "index_count":
			ai, bi = int64(a.IndexCount), int64(b.IndexCount)
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
