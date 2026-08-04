# chaos-lib 代码规则

> 本文件只列代码层面的硬规则。功能规格、变更规划、设计决策由 `openspec/` 管理。
> Go 通用风格（命名、错误处理、import 分组）遵循 Effective Go，此处不重复。

## 模块与路径

- 后端模块名 `chaos-go`，import 路径以 `chaos-go/` 开头
- 功能代码放 `internal/<module>/`，模型 + handler 同包

## 数据库

- 用 `config.GetDB()` 单例获取 DB，**不搞依赖注入**
- GORM 模型只加必要标签，**不写 `gorm:"column:xxx"`**（列名自动 snake_case）
- 软删除：字段统一 `IsDeleted bool`，查询手动加 `WHERE is_deleted = ?`
- 新模型必须在 `cmd/server/main.go` 的 `AutoMigrate()` 登记
- `chaos-go/sql/chaos_postgres_schema.sql` 是**手动导出的基准快照，AI 不得修改**
- 新增/修改 GORM 模型时，**必须写入 `chaos-go/sql/chaos_postgres_update.sql`**（变动日志形式，增量 DDL，紧凑迁移风格，非 pg_dump 导出格式）：建表（`id integer NOT NULL` + `IDENTITY` 序列）、表级与列级 `COMMENT`、主键约束、唯一/普通索引；保持与代码模型字段一致，注释用紧凑 `COMMENT ON` 写法而非 `-- Name:` 导出块。每条变动追加在文件末尾并注明日期与用途

## 后台任务

- 在包 `init()` 中 `scheduler.Register(name, interval, fn)` 注册

## 路由

- 在 `routes.SetupRouter(webFS embed.FS)` 注册，统一前缀 `/api`
- 前端经 `NoRoute` 回退到 `index.html`

## 依赖与平台

- `envvar` 与 `quickedit` 互相调用，通过 `main.go` 回调注入解循环依赖
- Junction / 注册表 / Windows 通知仅 Windows 平台

## 规格驱动开发（OpenSpec）

- 功能规格是真值源，放 `openspec/specs/<module>/spec.md`；变更提案放 `openspec/changes/<name>/`（proposal/design/tasks/specs 四件套）
- 工作流：`/opsx:propose` 提需求 → `/opsx:apply` 按 tasks.md 落地 → 补测试并**实际跑 `go test ./...`** → 用户确认可用后 **`/opsx:archive` 归档**
- 每次改动后主动提醒用户执行 `/opsx:archive` 关闭变更；未归档的 change 会污染后续提案列表
- spec.md 必须含 `## Purpose`、`## Requirements`（含 SHALL/MUST）、`#### Scenario:`（WHEN/THEN），否则 `openspec validate` 不过

## 提交前

- `go build ./...` 无编译错误
- `.env.dev` / `.env.prod` 不提交
