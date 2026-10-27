package dbmonitor

type TableStat struct {
	Name          string  `json:"name"`
	Rows          int64   `json:"rows"`
	RowsEstimated bool    `json:"rowsEstimated"`
	TableBytes    int64   `json:"tableBytes"`
	IndexBytes    int64   `json:"indexBytes"`
	TotalBytes    int64   `json:"totalBytes"`
	IndexCount    int     `json:"indexCount"`
	SizeSupported bool    `json:"sizeSupported"`
	SeqScan       *int64  `json:"seqScan,omitempty"`
	IdxScan       *int64  `json:"idxScan,omitempty"`
	LastVacuum    *string `json:"lastVacuum,omitempty"`
	LastAnalyze   *string `json:"lastAnalyze,omitempty"`
}

type DbOverview struct {
	DbType        string `json:"dbType"`
	Version       string `json:"version"`
	TotalBytes    int64  `json:"totalBytes"`
	TableCount    int    `json:"tableCount"`
	TotalRows     int64  `json:"totalRows"`
	SizeSupported bool   `json:"sizeSupported"`
}

type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	IsPK     bool    `json:"isPk"`
	Default  *string `json:"default,omitempty"`
}

type IndexInfo struct {
	Name    string `json:"name"`
	Unique  bool   `json:"unique"`
	Columns string `json:"columns"`
}

type TableDetail struct {
	TableStat
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}
