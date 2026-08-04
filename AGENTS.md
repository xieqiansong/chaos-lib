# chaos-lib 代码规则

> 本文件只列代码层面的硬规则。功能规格、变更规划、设计决策由 `openspec/` 管理。

## 数据库

- 用 `config.GetDB()` 单例获取 DB，**不搞依赖注入**
- GORM 模型只加必要标签，**不写 `gorm:"column:xxx"`**（列名自动 snake_case）
- 软删除：字段统一 `IsDeleted bool`，查询手动加 `WHERE is_deleted = ?`
- 新模型必须在 `cmd/server/main.go` 的 `AutoMigrate()` 登记
- `chaos-go/sql/chaos_postgres_schema.sql` 是**手动导出的基准快照，AI 不得修改**
- 新增/修改 GORM 模型时，**必须写入 `chaos-go/sql/chaos_postgres_update.sql`**（增量变动日志，参照文件内现有条目格式追加到末尾，注明日期与用途，保持与代码模型字段一致）

## 路由

- 在 `routes.SetupRouter(webFS embed.FS)` 注册，统一前缀 `/api`
- 前端经 `NoRoute` 回退到 `index.html`

## 规格驱动开发（OpenSpec）

- 功能规格是真值源，放 `openspec/specs/<module>/spec.md`；变更提案放 `openspec/changes/<name>/`（proposal/design/tasks/specs 四件套）
- 工作流：`/opsx:propose` 提需求 → `/opsx:apply` 按 tasks.md 落地 → 补测试并**实际跑 `go test ./...`** → 用户确认可用后 **`/opsx:archive` 归档**
- 每次改动后主动提醒用户执行 `/opsx:archive` 关闭变更；未归档的 change 会污染后续提案列表
- spec.md 必须含 `## Purpose`、`## Requirements`（含 SHALL/MUST）、`#### Scenario:`（WHEN/THEN），否则 `openspec validate` 不过
- 用户提需求后，主动提醒是否需要补充测试用例（后端配套 `_test.go`），由用户决定是否添加，不自动强写
- 涉及前端（chaos-ui）的改动不引入测试框架；change 的 tasks.md 必须附「前端手动验证清单」，由用户人工验收

## 提交前

- `go build ./...` 无编译错误
- `.env.dev` / `.env.prod` 不提交
