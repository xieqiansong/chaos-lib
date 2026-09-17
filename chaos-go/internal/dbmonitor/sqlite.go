package dbmonitor

import (
	"fmt"
	"os"
	"strings"

	"chaos-go/config"
	"gorm.io/gorm"
)

func sqliteDB() *gorm.DB { return config.GetDB() }

// dbstatAvailable 探测 SQLite 是否支持 dbstat 虚拟表（per-table 大小依赖它）。
func dbstatAvailable(db *gorm.DB) bool {
	var n int
	err := db.Raw("SELECT 1 FROM dbstat LIMIT 1").Scan(&n).Error
	return err == nil
}

func sqliteDBFileSize() int64 {
	info, err := os.Stat(config.GetConfig().Database.Path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func sqliteVersion(db *gorm.DB) string {
	var v string
	if err := db.Raw("SELECT sqlite_version()").Scan(&v).Error; err != nil {
		return ""
	}
	return v
}

func sqliteTableNames(db *gorm.DB) ([]string, error) {
	var rows []struct{ Name string }
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names, nil
}

func overviewSQLite() (*DbOverview, error) {
	db := sqliteDB()
	names, err := sqliteTableNames(db)
	if err != nil {
		return nil, err
	}
	tables, err := collectTablesSQLite()
	if err != nil {
		return nil, err
	}
	var totalRows int64
	for _, t := range tables {
		totalRows += t.Rows
	}
	return &DbOverview{
		DbType:        "sqlite",
		Version:       sqliteVersion(db),
		TotalBytes:    sqliteDBFileSize(),
		TableCount:    len(names),
		TotalRows:     totalRows,
		SizeSupported: dbstatAvailable(db),
	}, nil
}

func collectTablesSQLite() ([]TableStat, error) {
	db := sqliteDB()
	names, err := sqliteTableNames(db)
	if err != nil {
		return nil, err
	}
	sizeOK := dbstatAvailable(db)
	stats := make([]TableStat, 0, len(names))
	for _, name := range names {
		ts := TableStat{Name: name, SizeSupported: sizeOK}
		var rows int64
		if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&rows).Error; err == nil {
			ts.Rows = rows
		}
		if sizeOK {
			var tbl, idx int64
			db.Raw("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name=?", name).Scan(&tbl)
			db.Raw("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name IN (SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?)", name).Scan(&idx)
			ts.TableBytes = tbl
			ts.IndexBytes = idx
			ts.TotalBytes = tbl + idx
		}
		stats = append(stats, ts)
	}
	return stats, nil
}

func tableDetailSQLite(name string) (*TableDetail, error) {
	db := sqliteDB()
	var cnt int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name=?", name).Scan(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt == 0 {
		return nil, errTableNotFound
	}

	detail := &TableDetail{}
	detail.Name = name
	sizeOK := dbstatAvailable(db)

	// 列
	type colRow struct {
		Name  string  `gorm:"column:name"`
		Type  string  `gorm:"column:type"`
		Notn  int     `gorm:"column:notnull"`
		Dflt  *string `gorm:"column:dflt_value"`
		Pk    int     `gorm:"column:pk"`
	}
	var cols []colRow
	if err := db.Raw(fmt.Sprintf("PRAGMA table_info(%q)", name)).Scan(&cols).Error; err != nil {
		return nil, err
	}
	for _, c := range cols {
		detail.Columns = append(detail.Columns, ColumnInfo{
			Name: c.Name, Type: c.Type, Nullable: c.Notn == 0, IsPK: c.Pk > 0, Default: c.Dflt,
		})
	}

	// 索引
	type idxRow struct {
		Name   string `gorm:"column:name"`
		Unique int    `gorm:"column:unique"`
	}
	var idxs []idxRow
	if err := db.Raw(fmt.Sprintf("PRAGMA index_list(%q)", name)).Scan(&idxs).Error; err != nil {
		return nil, err
	}
	for _, ix := range idxs {
		var icols []struct {
			Name string `gorm:"column:name"`
		}
		db.Raw(fmt.Sprintf("PRAGMA index_info(%q)", ix.Name)).Scan(&icols)
		parts := make([]string, 0, len(icols))
		for _, ic := range icols {
			parts = append(parts, ic.Name)
		}
		detail.Indexes = append(detail.Indexes, IndexInfo{
			Name: ix.Name, Unique: ix.Unique != 0, Columns: strings.Join(parts, ","),
		})
	}
	detail.IndexCount = len(detail.Indexes)

	// 统计汇总
	if sizeOK {
		var tbl, idx int64
		db.Raw("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name=?", name).Scan(&tbl)
		db.Raw("SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name IN (SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?)", name).Scan(&idx)
		detail.TableBytes = tbl
		detail.IndexBytes = idx
		detail.TotalBytes = tbl + idx
	}
	detail.SizeSupported = sizeOK

	var rows int64
	db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&rows)
	detail.Rows = rows
	return detail, nil
}
