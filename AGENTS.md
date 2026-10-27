# chaos-lib 代码规则

> 只列硬性约束；与代码不符时以代码为准，并回改本文。

## 红线

- 公开仓库：禁止写入密码 / 密钥 / Token / API Key、内网地址与主机名、账号与个人身份信息、未脱敏日志与路径。
- 凭据只放 `configs/config.yaml` 或 `.env`（均已忽略）；`config.example.yaml` 只保留键名与占位值。
- `migrations/chaos_postgres_schema.sql` 是手动导出快照，禁止修改；模型变更只追加 `migrations/chaos_postgres_update.sql`（注日期与用途）。

## 后端

- 模块即插件：`internal/domain/<name>/`（外部集成放 `internal/platform/`）按 `handler` / `service` / `repository` / `model` / `plugin`（可选 `dto`）分文件；`plugin.go` 的 `init()` 调 `plugin.Register`，并在 `internal/app/app.go` 加 blank import。
- 依赖单向：`handler.go` → `service.go` → `repository.go` → `config.GetDB()`；`model.go` 不反向依赖任何层。
- `config.GetDB()` 只允许出现在 `repository.go`（framework 自身除外）；repository 只进出模型，DTO 转换在 handler / service。
- handler 只做参数解析、错误映射、响应组装；业务规则 / 编排 / IO / 事务放 `service.go`。
- 软删用 `crud.BaseModel`，判空用 `Deleted()`（`!x.IsDeleted` 无法编译）；仅 `db.Table(...)` 的联表 / 子查询需手写 `is_deleted = 0`。存量 PostgreSQL 须先应用 `chaos_postgres_update.sql` 里的 `is_deleted boolean → smallint` ALTER。
- 新模型登记到 `plugin.go` 的 `Models`，由 `config.AutoMigrate` 迁移；列名自动 snake_case，不写 `gorm:"column:..."`。
- 业务路由在 `RegisterV1(rg *web.RouterGroup)` 内挂载，不在 `router.go` / `routehub` 内联；handler 入参为 `*web.Context`（已无 gin）。
- 通用 CRUD 用 `crud.RegisterActions[T](rg, "<prefix>", crud.Opts[T]{...}, *crud.ToggleOpts)`；`Opts` / `ToggleOpts` 的回调体只转调 service。
- 业务动作：解析用 `envelope.Bind`，错误映射用 `httpx.MapError`，响应用 `renv.Success` / `renv.Error`。
- 后台任务分两类：插件自带的**内置任务**用 `plugin.go` 的 `Tasks []scheduler.TaskSpec` 声明，由 `framework/scheduler` 按 cron 运行（不落库，前端只读展示）；用户可编辑的 cron / HTTP / 命令任务走 `internal/domain/cronjob`。非周期常驻逻辑仍在 `Start` 里起 goroutine（互斥 + panic 恢复）。
- 循环依赖用 `Meta.SoftDeps` + 运行时函数变量注入，不在 `init()` 互相 import。
- 被重写 / 扩展的存量模块须迁移到上述基线。

## 前端

- 业务页面放 `src/plugins/<name>/`：`api.ts` 独占该资源的类型与请求函数（`useRestApi` + `action`），`.vue` 用 `DataTable`，`index.ts` 调 `registerPlugin` 声明路由；新接口不进 `src/utils/api.ts`（存量聚合）。
- 应用级页面才加到 `src/router/index.ts` 的 `appRoutes`。
- 请求走 `src/utils/request.ts`；提示 / 确认 / 输入弹窗走 `src/utils/message.ts`，页面不直接 import `ElMessage` / `ElMessageBox`。
- 写操作走 `useCrudAction().run(task, opts)`，不重写 try/catch + toast + refresh。
- 资源名后端 snake_case、前端 camelCase；时间列 `value-format="YYYY-MM-DDTHH:mm:ssZ"`。

## 构建

- 提交前 `go build ./...` 无错；后端逻辑改动跑 `go test ./...`；前端 `pnpm build`，产物拷到 exe 同目录 `ui/`（或 `CHAOS_UI_DIR`）。
