package dbmonitor

import "errors"

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
