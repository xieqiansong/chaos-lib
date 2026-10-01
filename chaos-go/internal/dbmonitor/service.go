package dbmonitor

import (
	"sort"
	"strings"

	"chaos-go/internal/config"
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

// ── 排序 ──

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
