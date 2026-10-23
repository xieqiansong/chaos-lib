# chaos-lib 代码规则

> 只列代码层面的硬规则。本文与 `README.md` 同步维护；若发现本文与代码不符，以代码为准并回改本文。

## 公开仓库红线

- 本仓库对外公开，**严禁写入敏感信息**：密码 / 密钥 / Token / API Key、内网 IP 与主机名、账号与个人身份信息、未脱敏日志与路径。
- 凭据只放 `configs/config.yaml`（`chaos-go/.gitignore` 已忽略）或 `.env`（均被忽略）；`configs/config.example.yaml` 只保留键名与占位值。

## 目录约定

- 后端 `chaos-go`（Go 1.27，module `chaos-go`）：
  - `internal/app/`：组合根，`app.go` 的 `Run` 编排启动（config → DB → blank import 各插件 → `plugin.Build` → `config.AutoMigrate` → `plugin.StartAll` → `router.SetupRouter`）。
  - `internal/framework/`：自研 web 框架 + 跨切面基础设施（均为非业务）。
    - `web/`：gin 兼容薄封装（替代 gin），`Context` / `Router` / `RouterGroup`。
    - `router/`：`SetupRouter(webFS fs.FS)` 装配入口。
    - `routehub/`：存量 v1 路由登记中心（`RegisterV1` / `MountAllV1`）。
    - `plugin/`：插件注册与依赖编排（`Register` / `Build` / `MountAll` / `StartAll`）。
    - `config/`：配置加载 + DB 单例 + 日志（包名仍为 `config`）。
    - `crud/`：通用 CRUD 基线（`BaseModel` / `RegisterActions` / `Opts` / `ToggleOpts`）。
    - `envelope/` + `resp/`（别名 `renv`）+ `httpx/`：请求信封 / 统一响应 / 错误映射。
    - `middleware/`：鉴权中间件；`pagination/` 分页；`scheduler/`（内置周期任务，原 `Register` 已废弃）；`apilog/`、`security/`、`dbmonitor/`、`datacache/`、`memcache/`。
  - `internal/domain/`：业务模块，每个都是一个插件（`handler`/`service`/`repository`/`model`/`plugin`，+`dto`），共 16 个（见「分层契约」末尾清单）。
  - `internal/platform/`：外部集成适配器（`deepseek` / `stunpf` / `stunsync` / `mqttsync`（插件）/ `supabase`）。
  - `pkg/tools/`：跨平台宿主交互工具（非业务：alarm/db/env_var/file_link/http/port_forwarding/project_fs/shell/uuid）。
  - `web/`：前端构建产物放置目录（`.gitkeep`）；运行期经 `CHAOS_UI_DIR` 或 exe 同目录 `ui/` 解析为 `webFS`。
  - `configs/`（config.yaml + 示例）、`migrations/`（SQL 基准 + 增量）、`deployments/`、`api/`（openapi 占位）、`scripts/`、`test/`。
- 前端 `chaos-ui`（Vue 3 + TS + Vite 5 + Element Plus + ECharts + Monaco，pnpm）：
  - `src/plugins/`：业务插件，与后端插件一一对应，每插件含 `api.ts`（接口门户）+ `.vue`（页面）+ 插件登记（`index.ts` 调 `registerPlugin`）。
  - `src/views/`：应用级页面（首页 / 测试例子 / 全屏看板）。
  - `src/components/`（DataTable 等通用组件）、`src/composables/`（useRestApi / useCrudAction）、`src/utils/`（request / message）、`src/router/`（appRoutes + `collectRoutes` 汇总插件路由）、`src/theme.ts`。

## 数据库

- 用 `config.GetDB()` 单例（现位于 `internal/framework/config`），**不搞依赖注入**。
- GORM 模型只加必要标签；列名自动 snake_case，**不写 `gorm:"column:xxx"`**。
- 软删除统一走 `gorm.io/plugin/soft_delete` 的 flag 模式：嵌入 `crud.BaseModel` 即自动获得（`IsDeleted soft_delete.DeletedAt`，`gorm:"softDelete:flag" json:"-"`）。插件在查询 / 更新 / 删除时自动追加 `is_deleted = 0`、`Delete()` 自动置 1，`Unscoped()` 可绕过。
  - **业务与 crud 基线不再手写 `is_deleted` 条件**；例外：`db.Table(...)` 的联表 / 子查询没有模型 schema，插件不生效，仍须显式写 `is_deleted = 0`。
  - 该字段底层是 uint，判空用 `crud.BaseModel` 提供的 `Deleted()`，写 `!x.IsDeleted` 无法编译。
  - 存量 PostgreSQL 库须先应用 `migrations/chaos_postgres_update.sql` 中对应的 `boolean → smallint` ALTER（插件以整数读写该列）；SQLite 以整数存储，无需变更。
- 新模型须在业务包 `plugin.go` 的 `Models` 字段登记，交给 `config.AutoMigrate` 统一迁移（不再在 `cmd/server/main.go` 逐个登记）。
- `migrations/chaos_postgres_schema.sql` 为手动导出基准快照，**AI 不得修改**；模型变动追加 `migrations/chaos_postgres_update.sql`（增量日志，注日期与用途）。

## 路由

- `router.SetupRouter(webFS fs.FS)`（`internal/framework/router/router.go`）装配，统一前缀 `/api/v1`：`apilog` 全局中间件 + `envelope.Middleware()` + `middleware.AuthMiddleware()`；`routehub.MountAllV1` 挂载存量路由、`plugin.MountAll` 挂载插件路由；`NoRoute` 回退 `index.html`。
- **已移除 gin**：handler 入参为 `*web.Context`、路由组 `*web.RouterGroup`（`internal/framework/web`，gin 兼容 API：`c.JSON` / `c.Param` / `c.ShouldBindJSON` / `c.Query` 等）；底层用 Go 1.22+ `net/http.ServeMux`，路径参数 `:name` / `*name` 自动转 `{name}` / `{name...}`。
- 业务路由在业务包 `RegisterV1(rg *web.RouterGroup)` 内挂载，由 `plugin.Register` 的 `RegisterRoutes` 回调触发；**禁止在 `router.go` / `routehub` 内联业务路由**。
- 前端新页面：业务功能放 `src/plugins/<name>/`（`<Name>.vue` + 插件登记里声明 `routes`），由 `collectRoutes()` 汇总进 `src/router/index.ts`；应用级页面在 `src/router/index.ts` 的 `appRoutes` 追加。`meta.title` / `meta.icon` 驱动菜单与面包屑（`hidden` 不进菜单，`fullscreen` 脱离布局）。

## 后台任务

- 旧 `scheduler.Register` 已废弃（包内 `Register` 入口已移除，仅保留 `Start` / `safeRun` 空转），**不要再用**。
- 后台任务统一走插件生命周期钩子：在业务包 `plugin.go` 的 `Start func() error` 里起 goroutine（含互斥 + panic 恢复）。
- 需要 cron 表达式 / 触发 HTTP、命令或系统动作的任务走 `internal/domain/cronjob`（robfig/cron，秒级 `SecondOptional`），由 `cronjob/plugin.go` 的 `Start` 钩子拉起；`service.go` / `executor.go` 负责执行与运行记录。

## 前端通用

- 请求统一走 `src/utils/request.ts`，**不引入 axios**；业务动作走 `action(prefix, name, data)`，列表走 `action(prefix, 'list', {}, meta)`（meta 含分页 / 排序 / 过滤 / like）。
- 提示 / 确认 / 输入弹窗统一走 `src/utils/message.ts`（`showSuccess` / `showError` / `showWarning` / `showInfo` / `confirm` / `prompt`），**页面不直接 import ElMessage/ElMessageBox**。
- 写操作（二次确认 → 请求 → 提示 → 关弹窗 / 刷新）统一走 `src/composables/useCrudAction.ts` 的 `run(task, opts)`，**页面不重复 try/catch + toast + refresh 骨架**。
- 路由 history 模式（`createWebHistory`）。
- 主题 `src/theme.ts`（`dark` 默认 / `paper` 浅黄护眼），持久化 key `chaos-ui:theme`。
- 编辑器统一 Monaco。
- 资源接口门户：`src/plugins/<name>/api.ts` 独占该资源类型与 api 函数（`useRestApi<T>(prefix)` + `action`），**禁止聚合进 `utils/api.ts`**（该文件不存在，勿新建）。

## 业务模块脚手架基线

新增 / 重写业务模块一律套用本基线（= 一个「插件」）。参考实现：`internal/domain/items/` + `src/plugins/items/`（api.ts + Items.vue）。后端包内文件职责见下一节「分层契约」。

- **后端**：在 `internal/domain/<name>/` 下按 `handler` / `service` / `repository` / `model` / `plugin` 分层（DTO 收进 `dto.go`）。
  - `plugin.go`：`init() { plugin.Register(plugin.Registration{ Meta:{ID,Name,Version,Dependencies,SoftDeps,Disabled}, Models:[]any{&X{}}, RegisterRoutes: func(rg){ RegisterV1(rg) }, Start: ... }) }`；并在 `internal/app/app.go` 顶部加 blank import `_ "chaos-go/internal/domain/<name>"`。
  - 模型嵌入 `crud.BaseModel`，`RegisterV1` 里 `crud.RegisterActions[T](rg, "<name>", crud.Opts[T]{Searchable,Sortable,Protected}, &crud.ToggleOpts{Setter, ErrRules})` 生成通用 CRUD（POST + Action：`list` / `get` / `create` / `update` / `delete` / `batchCreate` / `batchDelete` / `status`，HTTP 恒 200，响应走 envelope）。启停改用第 4 参数 `ToggleOpts`，原 `crud.RegisterToggle` 已废弃。
  - 业务动作在 `RegisterV1` 内 `g.POST("/action", actionHandler)` 挂接；解析用 `envelope.Bind`、错误映射用 `httpx.MapError(c, err, errRules)`、响应用 `renv.Success` / `renv.Error`；路径参数解析与领域错误映射一律走 `internal/httpx`（`ParseID` / `ParseParam` / `MapError` + `ErrRule` 表）。
- **前端**：`src/plugins/<name>/api.ts` 定义 `interface` + `useRestApi<Resource>('<name>')` + `action('<name>', 'xxx', data)`；`src/plugins/<name>/<Name>.vue` 用通用 `DataTable`，仅声明 `columns`，无增删改查样板；同时在插件登记里声明 `routes`（含 `meta.title` / `meta.icon`）。
- **约定**：资源名后端 snake_case、前端 camelCase；时间列须 RFC3339 带时区 `value-format="YYYY-MM-DDTHH:mm:ssZ"`，否则 Go `time.Time` 反序列化报错。

> 存量聚合代码（旧模块）为历史遗留，不强制整改；被重写 / 扩展的模块须迁移到本基线。

## 分层契约（后端业务包）

`internal/domain/<name>/`（或 `internal/platform/<name>/`）内按文件划分职责层（同包不同层），**依赖只能单向向下**：

`handler.go` → `service.go` → `repository.go` → `config.GetDB()`

`model.go` 可被任意层依赖，自身不依赖任何层。

| 文件 | 职责 | 允许 | 禁止 |
|---|---|---|---|
| `handler.go` | HTTP 适配：解析参数、调 service、组装响应、`RegisterV1` 挂载路由 | `web.Context`、状态码映射、调用 service / repository 的只读查询 | 业务规则与分支编排、直接 `config.GetDB()`、直接文件 / 网络 IO |
| `service.go` | 用例编排与领域规则 | 调 repository、事务编排、跨模块调用、派生值计算 | `web.Context`、`renv.Success/Error`、裸 SQL、HTTP 中间件 |
| `repository.go` | **包内唯一数据访问出口** | GORM 查询、软删过滤、分页 | 返回 DTO、业务判断、`web.Context`、非数据访问的副作用 |
| `model.go` | 实体 + 领域不变量 | 结构体、`TableName()`、不碰 IO 的纯函数（校验 / 规范化 / 地址推导） | 任何 IO、引用 service 层全局状态（调度器 / 转发器 / 连接状态）、依赖 handler |
| `dto.go`（可选） | 请求 / 响应契约 | 具名 `XxxRequest` / `XxxResponse` 与 `toResponse` 组装 | 业务规则 |
| `plugin.go` | 插件登记（`init` 调 `plugin.Register`） | 声明 Meta / Models / RegisterRoutes / Start / Stop | 业务实现（只登记，不写逻辑） |

硬规则：

1. **DB 单一入口**：业务包内 `config.GetDB()` 只允许出现在 `repository.go`（`internal/framework` 基线自身除外）。`init()` 不得播种数据或起后台 goroutine，启动副作用放 `seed.go` / `worker.go` / `scheduler.go`，由 `plugin.go` 的 `Start` 钩子或 `app.go` 显式调用。
2. **model 不反向依赖**：需要运行状态（如"隧道是否在线"）时由调用方取好再传入，不在 model 里读全局运行时。
3. **repository 只进出模型**：返回 `[]Entity` / `*Entity`；DTO 转换在 handler 或 service 完成。
4. **handler 不承载业务**：出现 `if` 业务规则、循环编排、磁盘 / 网络副作用、事务控制，一律下沉 `service.go`（handler 只留参数解析 + 错误码映射 + 响应组装）。
5. **crud 回调只做转发**：`Opts[T]` 的 `Before/AfterXxx` 回调体只写一行转调 service，业务实现放 `service.go`；禁止在 `RegisterActions` 里内联闭包写业务逻辑。
6. **不在事务内做不可回滚的副作用**：建目录、删联接点等要么移到事务提交后，要么改成可补偿流程。
7. **平台实现 / 外部客户端 / 运行时各自独立成文件**：`xxx_windows.go` + `xxx_other.go`、`client.go`（SSH / MQTT / 上游 HTTP）、`scheduler.go` / `forwarder.go`（长驻运行时，由 `plugin.Start` 拉起）、`seed.go`（默认数据）、`middleware.go`（ web 中间件）。它们都**不是** `service.go`。
8. **优先具名 DTO**：响应优先用 `model.go` / `dto.go` 里的具名结构体；`map` 仅限临时聚合视图。
9. **插件登记与依赖**：循环依赖用 `Meta.SoftDeps`（接口型，运行时通过包级函数变量注入解环，如 envvar↔quickedit），不在 `init()` 里直接 import 互相依赖；硬依赖在 `Dependencies` 声明，交给 `plugin.Build` 做拓扑序 + 缺失 / 环检测（fail-fast）。

参考实现：`internal/domain/items/`（handler 纯注册 + 业务动作、model 纯实体、repository 纯查询、plugin 登记）。

> 存量模块为历史遗留，不强制整改；**被重写或扩展时必须迁移到本契约与插件基线**。当前插件化清单（共 19 个）：
> - `internal/domain`（16）：`items`、`dict`、`envvar`、`filelink`、`quickedit`、`taskplan`、`project`、`portfwd`、`proxy`、`sdk`、`polyform`、`standarddata`、`systemjob`、`cronjob`、`dashboard`、`sshconn`
> - `internal/platform`（1）：`mqttsync`
> - `internal/framework`（2）：`dbmonitor`、`datacache`

## 构建与部署

- 提交前 `go build ./...` 无错；后端逻辑改动后跑 `go test ./...`。
- 前端 `pnpm build`（在 `chaos-ui`）产出 `dist/`；`scripts/chaos.deploy.ps1` 拷到 exe 同目录 `ui/`。后端**不内嵌前端**，启动时从 `CHAOS_UI_DIR` 或 `./ui` 读静态资源（经 `cmd/server/main.go` 解析为 webFS）；前端改动只需重拷 `dist/`，无需重编二进制。
