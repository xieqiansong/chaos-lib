package dbmonitor

import (
	"fmt"

	"chaos-go/config"
	"gorm.io/gorm"
)

func postgresDB() *gorm.DB { return config.GetDB() }

type pgListRow struct {
	Name        string  `gorm:"column:name"`
	TableBytes  int64   `gorm:"column:table_bytes"`
	IndexBytes  int64   `gorm:"column:index_bytes"`
	TotalBytes  int64   `gorm:"column:total_bytes"`
	IndexCount  int     `gorm:"column:index_count"`
	SeqScan     *int64  `gorm:"column:seq_scan"`
	IdxScan     *int64  `gorm:"column:idx_scan"`
	LastVacuum  *string `gorm:"column:last_vacuum"`
	LastAnalyze *string `gorm:"column:last_analyze"`
}

const pgListSQL = `
SELECT
  c.relname AS name,
  pg_table_size(c.oid) AS table_bytes,
  pg_indexes_size(c.oid) AS index_bytes,
  pg_total_relation_size(c.oid) AS total_bytes,
  (SELECT count(*) FROM pg_index WHERE indrelid = c.oid) AS index_count,
  s.seq_scan, s.idx_scan,
  s.last_vacuum::text, s.last_analyze::text
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_stat_user_tables s ON s.relid = c.oid
WHERE c.relkind = 'r' AND n.nspname = current_schema()
ORDER BY c.relname`

func overviewPostgres() (*DbOverview, error) {
	db := postgresDB()
	var version string
	if err := db.Raw("SELECT version()").Scan(&version).Error; err != nil {
		return nil, err
	}
	var totalBytes int64
	db.Raw("SELECT pg_database_size(current_database())").Scan(&totalBytes)
	tables, err := collectTablesPostgres()
	if err != nil {
		return nil, err
	}
	var totalRows int64
	for _, t := range tables {
		totalRows += t.Rows
	}
	return &DbOverview{
		DbType:        "postgres",
		Version:       version,
		TotalBytes:    totalBytes,
		TableCount:    len(tables),
		TotalRows:     totalRows,
		SizeSupported: true,
	}, nil
}

func collectTablesPostgres() ([]TableStat, error) {
	db := postgresDB()
	var rows []pgListRow
	if err := db.Raw(pgListSQL).Scan(&rows).Error; err != nil {
		return nil, err
	}
	stats := make([]TableStat, 0, len(rows))
	for _, r := range rows {
		ts := TableStat{
			Name:          r.Name,
			RowsEstimated: false,
			TableBytes:    r.TableBytes,
			IndexBytes:    r.IndexBytes,
			TotalBytes:    r.TotalBytes,
			IndexCount:    r.IndexCount,
			SizeSupported: true,
			SeqScan:       r.SeqScan,
			IdxScan:       r.IdxScan,
			LastVacuum:    r.LastVacuum,
			LastAnalyze:   r.LastAnalyze,
		}
		var cnt int64
		if err := db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM %q`, r.Name)).Scan(&cnt).Error; err == nil {
			ts.Rows = cnt
		}
		stats = append(stats, ts)
	}
	return stats, nil
}

func tableDetailPostgres(name string) (*TableDetail, error) {
	db := postgresDB()
	var cnt int64
	if err := db.Raw("SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND c.relname=? AND n.nspname=current_schema()", name).Scan(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt == 0 {
		return nil, errTableNotFound
	}

	detail := &TableDetail{}
	detail.Name = name
	detail.SizeSupported = true

	// 列
	type colRow struct {
		Name     string  `gorm:"column:column_name"`
		Type     string  `gorm:"column:data_type"`
		Nullable bool    `gorm:"column:is_nullable"`
		Default  *string `gorm:"column:column_default"`
	}
	var cols []colRow
	if err := db.Raw(`SELECT column_name, data_type, (is_nullable='YES') AS is_nullable, column_default
		FROM information_schema.columns WHERE table_name=? AND table_schema=current_schema() ORDER BY ordinal_position`, name).Scan(&cols).Error; err != nil {
		return nil, err
	}
	pkSet := map[string]bool{}
	var pks []string
	if err := db.Raw(`SELECT a.attname FROM pg_index ix
		JOIN pg_attribute a ON a.attrelid=ix.indrelid AND a.attnum=ANY(ix.indkey)
		WHERE ix.indrelid=(?::regclass) AND ix.indisprimary`, name).Scan(&pks).Error; err == nil {
		for _, p := range pks {
			pkSet[p] = true
		}
	}
	for _, c := range cols {
		detail.Columns = append(detail.Columns, ColumnInfo{
			Name: c.Name, Type: c.Type, Nullable: c.Nullable, IsPK: pkSet[c.Name], Default: c.Default,
		})
	}

	// 索引
	type idxRow struct {
		IndexName string `gorm:"column:index_name"`
		Unique    bool   `gorm:"column:indisunique"`
		Cols      string `gorm:"column:cols"`
	}
	var idxs []idxRow
	if err := db.Raw(`SELECT i.relname AS index_name, ix.indisunique,
		(SELECT string_agg(a.attname, ',' ORDER BY arr.i) FROM unnest(ix.indkey) WITH ORDINALITY AS arr(k,i)
			JOIN pg_attribute a ON a.attrelid=ix.indrelid AND a.attnum=arr.k) AS cols
		FROM pg_index ix JOIN pg_class i ON i.oid=ix.indexrelid
		JOIN pg_class t ON t.oid=ix.indrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE t.relname=? AND n.nspname=current_schema()`, name).Scan(&idxs).Error; err != nil {
		return nil, err
	}
	for _, ix := range idxs {
		detail.Indexes = append(detail.Indexes, IndexInfo{Name: ix.IndexName, Unique: ix.Unique, Columns: ix.Cols})
	}
	detail.IndexCount = len(detail.Indexes)

	// 统计
	var row pgListRow
	if err := db.Raw(`SELECT c.relname AS name,
		pg_table_size(c.oid) AS table_bytes,
		pg_indexes_size(c.oid) AS index_bytes,
		pg_total_relation_size(c.oid) AS total_bytes,
		(SELECT count(*) FROM pg_index WHERE indrelid=c.oid) AS index_count,
		s.seq_scan, s.idx_scan, s.last_vacuum::text, s.last_analyze::text
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		LEFT JOIN pg_stat_user_tables s ON s.relid=c.oid
		WHERE c.relkind='r' AND c.relname=? AND n.nspname=current_schema()`, name).Scan(&row).Error; err == nil {
		detail.TableBytes = row.TableBytes
		detail.IndexBytes = row.IndexBytes
		detail.TotalBytes = row.TotalBytes
		detail.IndexCount = row.IndexCount
		detail.SeqScan = row.SeqScan
		detail.IdxScan = row.IdxScan
		detail.LastVacuum = row.LastVacuum
		detail.LastAnalyze = row.LastAnalyze
	}

	var cnt2 int64
	db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM %q`, name)).Scan(&cnt2)
	detail.Rows = cnt2
	return detail, nil
}
