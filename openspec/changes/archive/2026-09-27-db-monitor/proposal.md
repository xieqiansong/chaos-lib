# Change: 数据库监控模块（db-monitor）

## Why

`chaos-lib` 同时以 SQLite（默认）与 PostgreSQL 作为后端，但当前没有任何手段直观查看库内有哪些表、各表多大、有多少行、索引与统计信息如何。本变更新增一个只读自省模块，帮助运维/排障时快速掌握数据库体量。

## What Changes

- 新增后端包 `internal/dbmonitor`，按 `config.Database.Type` 分 SQLite / PostgreSQL 两套实现，统一产出 `TableStat` 结构。
- 新增三个只读 API：`/api/dbMonitor/overview`、`/api/dbMonitor/tables`、`/api/dbMonitor/tables/:name`。
- 前端新增「数据库监控」页面（`/databaseMonitor`），展示概览卡 + 可排序表列表 + 详情抽屉。
- 纯只读，不新增任何业务表，无需 AutoMigrate 与 `sql/` 变更日志。

## Impact

- 仅新增代码，不改动既有接口与数据模型。
- 监控接口会读取数据库系统目录，属只读操作，无写入风险。
