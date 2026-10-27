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

	if detail.Columns == nil {
		detail.Columns = []ColumnInfo{}
	}
	if detail.Indexes == nil {
		detail.Indexes = []IndexInfo{}
	}
	return detail, nil
}

func Overview() (*DbOverview, error) {
	return getOverview()
}

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
		default:
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
