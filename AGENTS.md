# chaos-lib 代码规则

> 本文件只列代码层面的硬规则。功能规格、变更规划、设计决策由 `openspec/` 管理。

## 公开仓库红线

- 本仓库对外公开，**严禁写入敏感信息**：密码 / 密钥 / Token / API Key、内网 IP 与主机名、账号与个人身份信息、未脱敏的日志与路径。
- 凭据只放 `.env` / `.env.dev` / `.env.prod`（均被 `.gitignore` 忽略）；`.env.example` 只保留键名与占位值。
- 涉及上述信息的示例、日志、路径一律脱敏或删减。

## 目录约定

- 后端 `chaos-go`（module `chaos-go`，Go 1.26），业务逻辑一律放 `internal/<模块>/`，模型 + handler 同包。
- 基础设施：`config/`（配置 + DB 单例 + 日志）、`routes/`（路由注册）、`scheduler/`（后台调度）、`cmd/`（入口）。
- `models/`、`tools/`、`tasks/`、`_archived/` 为遗留实现，**新改动一律落在 `internal/`**，不在这些目录扩展。
- 前端 `chaos-ui`（Vue 3 + TypeScript + Vite 5 + Element Plus，包管理器 pnpm）。

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
- 新增前端页面：在 `chaos-ui/src/router/index.ts` 的 `appRoutes` 追加一条路由，`meta.title` / `meta.icon` 即菜单与面包屑来源，菜单自动生成（`meta.hidden` 不进菜单，`meta.fullscreen` 脱离常规布局）

## 后台任务

- 需要周期任务时，在对应包的 `init()` 中调用 `scheduler.Register(name, interval, fn, enabledFn...)`；调度器每任务独立 goroutine，含互斥与 panic 恢复

## 前端

- 请求统一走 `chaos-ui/src/utils/api.ts`（基于原生 fetch，`API_BASE = VITE_API_BASE || '/api'`，带重试），**不引入 axios**
- 路由为 hash 模式（`createWebHashHistory`）
- 主题由 `chaos-ui/src/theme.ts` 管理（`dark` 默认 / `paper` 浅黄护眼），持久化 key `chaos-ui:theme`
- 编辑器统一用 Monaco（`predev` / `prebuild` 会把 `node_modules/monaco-editor/min` 拷到 `public/assets/monaco/min`）

## 规格驱动开发（OpenSpec）

- 功能规格是真值源，放 `openspec/specs/<module>/spec.md`；变更提案放 `openspec/changes/<name>/`（proposal/design/tasks/specs 四件套）
- 工作流：`/opsx:propose` 提需求 → `/opsx:apply` 按 tasks.md 落地 → 补测试并**实际跑 `go test ./...`** → 用户确认可用后 **`/opsx:archive` 归档**
- 每次改动后主动提醒用户执行 `/opsx:archive` 关闭变更；未归档的 change 会污染后续提案列表
- spec.md 必须含 `## Purpose`、`## Requirements`（含 SHALL/MUST）、`#### Scenario:`（WHEN/THEN），否则 `openspec validate` 不过
- 用户提需求后，主动提醒是否需要补充测试用例（后端配套 `_test.go`），由用户决定是否添加，不自动强写
- 涉及前端（chaos-ui）的改动不引入测试框架；change 的 tasks.md 必须附「前端手动验证清单」，由用户人工验收

## 构建与部署

- 提交前必须 `go build ./...` 无编译错误；后端逻辑改动后跑 `go test ./...`
- 前端 `pnpm build` 产出 `dist/`，通过 `scripts/chaos.deploy.ps1` 拷到 `chaos-go/cmd/server/web` 并重建二进制（`//go:embed web` 相对 `cmd/server/main.go` 解析）
