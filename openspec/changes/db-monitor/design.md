# Design: 数据库监控模块

## 双后端分实现

`chaos-lib` 后端由 `config.GetDB()` 单例提供 GORM 实例，数据库类型存于 `config.GetConfig().Database.Type`。
模块在 `dbmonitor.go` 中以 `isSQLite()` 分派到 `sqlite.go` / `postgres.go`，对外暴露统一结构：

```go
type TableStat struct {
    Name          string
    Rows          int64
    RowsEstimated bool   // true=统计估值, false=COUNT(*) 精确
    TableBytes    int64
    IndexBytes    int64
    TotalBytes    int64
    IndexCount    int
    SizeSupported bool   // SQLite 无 dbstat 时降级为 false
    // PostgreSQL 专属（SQLite 下省略）：SeqScan / IdxScan / LastVacuum / LastAnalyze
}
```

## SQLite 实现要点

- 表清单：`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`。
- 行数：`SELECT COUNT(*) FROM "<name>"`（SQLite 无可靠估值，直接 COUNT，表通常不大）。
- 库总大小：`os.Stat(config.GetConfig().Database.Path)` 取文件字节（最准）。
- 单表大小：探测 `dbstat` 虚拟表（`SELECT 1 FROM dbstat LIMIT 1`）；可用则 `SUM(pgsize)` 区分表页与索引页；**不可用（modernc.org/sqlite 默认未编译该扩展）时 `SizeSupported=false`**，前端显示 N/A，不报错。

## PostgreSQL 实现要点

- 表清单 + 大小 + 统计：一条 `pg_class` 联 `pg_namespace` + `pg_stat_user_tables` 的查询，使用 `pg_table_size / pg_indexes_size / pg_total_relation_size(oid)` 与 `n_live_tup`、`seq_scan` 等。
- 行数：默认取 `n_live_tup` 估值（`RowsEstimated=true`）；`?exact=true` 时逐表 `COUNT(*)` 取精确值。
- 详情：列来自 `information_schema.columns` + `pg_index` 主键；索引来自 `pg_index`/`pg_class` 解析列名。

## 安全

- `:name` 入参先经正则 `^[A-Za-z_][A-Za-z0-9_]*$` 校验，且必须在系统目录中真实存在才执行自省，杜绝 SQL 注入。
- 表名一律用双引号包裹（SQLite/PG 通用），不走参数占位符（系统目录不支持标识符绑定）。
