# chaos-lib 代码规则

> 本文件只列代码层面的硬规则。功能规格、变更规划、设计决策由 `openspec/` 管理。

## 公开仓库红线

- 本仓库对外公开，**严禁写入敏感信息**：密码 / 密钥 / Token / API Key、内网 IP 与主机名、账号与个人身份信息、未脱敏日志与路径。
- 凭据只放 `.env` / `.env.dev` / `.env.prod`（均被 `.gitignore` 忽略）；`.env.example` 只保留键名与占位值。

## 目录约定

- 后端 `chaos-go`（Go 1.26，module `chaos-go`）：业务逻辑一律放 `internal/<模块>/`，模型 + handler 同包。
- 基础设施：`config/`（配置 + DB 单例 + 日志）、`routes/`（路由注册）、`scheduler/`（后台调度）、`cmd/`（入口）。
- `models/`、`tools/`、`tasks/`、`_archived/` 为非现役目录，**新改动一律落在 `internal/`**。
- 前端 `chaos-ui`（Vue 3 + TS + Vite 5 + Element Plus，pnpm）。

## 数据库

- 用 `config.GetDB()` 单例，**不搞依赖注入**。
- GORM 模型只加必要标签，**不写 `gorm:"column:xxx"`**（列名自动 snake_case）。
- 软删除统一 `IsDeleted bool`，查询手动加 `WHERE is_deleted = ?`。
- 新模型须在 `cmd/server/main.go` 的 `AutoMigrate()` 登记。
- `sql/chaos_postgres_schema.sql` 为手动导出基准快照，**AI 不得修改**。
- 模型变动须追加 `sql/chaos_postgres_update.sql`（增量日志，注日期与用途，字段与代码一致）。

## 路由

- `routes.SetupRouter(webFS fs.FS)` 注册，统一前缀 `/api`；前端 `NoRoute` 回退 `index.html`。
- 前端新页面在 `src/router/index.ts` 的 `appRoutes` 追加；`meta.title/icon` 驱动菜单与面包屑（`hidden` 不进菜单，`fullscreen` 脱离布局）。
- 业务路由须走业务包 `Register(api)`，**禁止在 `routes.go` 内联**。

## 后台任务

- 周期任务在对应包 `init()` 调 `scheduler.Register(name, interval, fn, enabledFn...)`；调度器每任务独立 goroutine（含互斥 + panic 恢复）。

## 前端通用

- 请求统一走 `src/utils/api.ts`（原生 fetch，`API_BASE = VITE_API_BASE || '/api'`，带重试），**不引入 axios**。
- 路由 hash 模式（`createWebHashHistory`）。
- 主题 `src/theme.ts`（`dark` 默认 / `paper` 浅黄护眼），持久化 key `chaos-ui:theme`。
- 编辑器统一 Monaco（`predev`/`prebuild` 拷 `node_modules/monaco-editor/min` → `public/assets/monaco/min`）。
- 接口门户：`src/api/<resource>.ts` 独占该资源类型与 api 函数（只借 `@/utils/api` 的 `sendMessage`），**禁止聚合进 `utils/api.ts`**。

## 业务模块脚手架基线

新增 / 重写业务模块（不论 CRUD 还是只读）一律套用本基线；参考实现：`internal/standarddata/` + `src/views/StandardData.vue` + `src/api/standardData.ts`。

- **后端**：模型在 `internal/<模块>/<资源>.go`，CRUD 资源嵌入 `crud.BaseModel` 并走 `crud.Register`（反射生成 list/get/create/update/delete 五路由，免写标准 handler）；实现 `TableName()` 与 `Register(rg *gin.RouterGroup)`，`routes.go` 只写 `<pkg>.Register(api)`；扩展能力（如状态切换）在 `Register` 内自定义路由挂载。
- **前端**：`src/api/<resource>.ts` 定义 `interface` + `export const <resource>Api = useRestApi<Resource>('<resource>')`；`src/views/<Resource>.vue` 用通用 `DataTable`，仅声明 `columns`（与 CRUD 时的 `fields`），无增删改查样板，自动内置操作列与弹窗。
- **约定**：资源名前端 camelCase（`standardData`）、表名 snake_case（`standard_datas`）；时间列须 RFC3339 带时区 `value-format="YYYY-MM-DDTHH:mm:ssZ"`，否则 Go `time.Time` 反序列化报错。

> 存量聚合代码（旧模块）为历史遗留不强制整改；被重写 / 扩展的模块须迁移到本基线。

## OpenSpec 规格驱动开发

- 规格为真值源：`openspec/specs/<module>/spec.md`；变更提案 `openspec/changes/<name>/`（proposal/design/tasks/specs 四件套）。
- 工作流：`/opsx:propose` → `/opsx:apply`（按 tasks.md）→ 补测试并**实跑 `go test ./...`** → 确认后 `/opsx:archive`。
- 每次改动后提醒用户 `/opsx:archive` 关闭变更；未归档 change 会污染提案列表。
- `spec.md` 须含 `## Purpose`、`## Requirements`（SHALL/MUST）、`#### Scenario:`（WHEN/THEN），否则 `openspec validate` 不过。
- 后端改动主动提醒补 `_test.go`（用户决定）；前端改动不引入测试框架，tasks.md 须附「前端手动验证清单」人工验收。

## 构建与部署

- 提交前 `go build ./...` 无错；后端逻辑改动后跑 `go test ./...`。
- 前端 `pnpm build` 产出 `dist/`；`scripts/chaos.deploy.ps1` 拷到 exe 同目录 `ui/`。后端**不内嵌前端**，启动时从 `CHAOS_UI_DIR` 或 `./ui` 读静态资源；前端改动只需重拷 `dist/`，无需重编二进制。
