# Spec: Database Monitor (数据库监控)

## Purpose

提供数据库只读自省能力，让使用者在不离开应用的情况下查看当前库内有哪些表、各表的行数、占用大小（表/索引/合计）、索引数量，以及（PostgreSQL 下）扫描与维护统计。模块 MUST 兼容 SQLite 与 PostgreSQL 双后端，且 MUST 不影响既有业务数据。

## Requirements

### Requirement: Database overview
系统 SHALL 提供库级总览，包含数据库类型、版本、总大小、表数量与总行数。

#### Scenario: Get overview
- **WHEN** 请求 `GET /api/dbMonitor/overview`
- **THEN** 返回 `{ dbType, version, totalBytes, tableCount, totalRows, sizeSupported }`

#### Scenario: SQLite total size is file size
- **WHEN** 后端为 SQLite
- **THEN** `totalBytes` 为数据库文件字节数，`sizeSupported` 反映 dbstat 是否可用

### Requirement: List user tables with statistics
系统 SHALL 列出全部用户表，并给出每表的行数（精确 COUNT(*))、大小、索引数等统计。

#### Scenario: List tables
- **WHEN** 请求 `GET /api/dbMonitor/tables`
- **THEN** 返回 `{ items: TableStat[], total }`，不含 `sqlite_` 内部表，行数为实际 COUNT(*)

#### Scenario: Sort tables
- **WHEN** 列表带 `?sort=size|rows|name&order=asc|desc`
- **THEN** 按指定字段与方向排序，缺省按名称升序

#### Scenario: Row count is exact
- **WHEN** 请求 `GET /api/dbMonitor/tables`（SQLite 或 PostgreSQL）
- **THEN** 每表 `rows` 为 `COUNT(*)` 精确值，`rowsEstimated` 恒为 false

### Requirement: Table detail
系统 SHALL 提供单表详情，包含列定义、索引列表与维护统计。

#### Scenario: Get table detail
- **WHEN** 请求 `GET /api/dbMonitor/tables/:name`（表存在）
- **THEN** 返回统计 + `columns`（列名/类型/可空/主键/默认值）+ `indexes`（索引名/唯一/列）

#### Scenario: Unknown table
- **WHEN** `:name` 对应的表不存在
- **THEN** 返回 404

#### Scenario: Invalid table name
- **WHEN** `:name` 含非法字符（非 `^[A-Za-z_][A-Za-z0-9_]*$`）
- **THEN** 返回 400，不执行任何 SQL

### Requirement: SQLite size degradation
系统 SHALL 在 SQLite 不支持 dbstat 虚拟表时，将单表大小相关字段降级为不可用而非报错。

#### Scenario: dbstat unavailable
- **WHEN** SQLite 后端未编译 dbstat
- **THEN** 各表 `sizeSupported=false`、大小字段为 0，接口正常返回 200

## API Endpoints (current)

```
GET /api/dbMonitor/overview          库级总览
GET /api/dbMonitor/tables            表统计列表（?sort/?order/?exact）
GET /api/dbMonitor/tables/:name      单表详情
```

## Out of Scope
- 写入 / 修改表结构（本模块纯只读）。
- 索引级明细、库级健康度评分、写入增长趋势（后续可扩展）。
- 跨 schema / 跨数据库实例的对比。
