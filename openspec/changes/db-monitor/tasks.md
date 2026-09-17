# Tasks: 数据库监控模块

## 后端（chaos-go）

- [x] 新建 `internal/dbmonitor/dbmonitor.go`：统一结构（`TableStat`/`DbOverview`/`ColumnInfo`/`IndexInfo`/`TableDetail`）、handler（`GetOverview`/`ListTables`/`GetTableDetail`）、按 DB 类型分派、排序、表名校验。
- [x] 新建 `internal/dbmonitor/sqlite.go`：表清单、库/表大小（dbstat 探测与降级）、行数、列与索引自省、详情。
- [x] 新建 `internal/dbmonitor/postgres.go`：表清单+大小+统计、估值/精确行数、列/主键/索引详情。
- [x] 在 `routes/routes.go` 注册 `/api/dbMonitor/overview`、`/api/dbMonitor/tables`、`/api/dbMonitor/tables/:name`。
- [x] 新增 `internal/dbmonitor/dbmonitor_test.go`（表名校验 + 排序纯逻辑测试）。
- [x] `go build ./...` 通过；`go test ./internal/dbmonitor/...` 通过。

## 前端（chaos-ui）

- [x] 新建 `src/views/DatabaseMonitor.vue`：概览卡、可排序/可筛选表列表、详情抽屉（列/索引/PG 统计）。
- [x] `src/router/index.ts` 追加 `/databaseMonitor` 路由（菜单自动出现）。
- [x] `src/utils/api.ts` 追加 `getDbOverview`/`getTables`/`getTableDetail`。
- [x] `pnpm build` 通过。

## 前端手动验证清单（由用户人工验收）

- [ ] 启动后端（SQLite 默认），打开 `/databaseMonitor`，4 张概览卡显示 DB 类型为 sqlite、版本号、文件总大小、表数/行数。
- [ ] 表列表可点列头排序（名称/行数/表大小/索引大小/总大小），关键字筛选生效。
- [ ] SQLite 下「表大小/索引大小/总大小」列为 N/A（dbstat 未启用），概览显示降级提示。
- [ ] 点击某行弹出抽屉，列定义（含主键★、可空、默认值）与索引（含唯一标记）正确显示。
- [ ] 切到 PostgreSQL（`DB_TYPE=postgres`）后，大小列正常显示，抽屉额外出现 seq_scan/idx_scan/最近 vacuum·analyze。
- [ ] PostgreSQL 下所有表行数为非零精确值（不再出现全 0 估值失真），列表无「估」标签。
- [ ] 非法表名（如含空格/分号）接口返回 400；不存在的表返回 404。
