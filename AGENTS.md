# chaos-lib 代码规则

> 只列代码层面的硬规则。

## 公开仓库红线

- 本仓库对外公开，**严禁写入敏感信息**：密码 / 密钥 / Token / API Key、内网 IP 与主机名、账号与个人身份信息、未脱敏日志与路径。
- 凭据只放 `.env` / `.env.dev` / `.env.prod`（均被 `.gitignore` 忽略）；`.env.example` 只保留键名与占位值。

## 目录约定

- 后端 `chaos-go`（Go 1.26，module `chaos-go`）：业务逻辑一律放 `internal/<模块>/`，模型 + handler 同包。
- 基础设施：`config/`（配置 + DB 单例 + 日志）、`routes/`（路由注册）、`scheduler/`（内置周期任务）、`cmd/`（入口）。
- 前端 `chaos-ui`（Vue 3 + TS + Vite 5 + Element Plus，pnpm）。

## 数据库

- 用 `config.GetDB()` 单例，**不搞依赖注入**。
- GORM 模型只加必要标签；列名自动 snake_case，**不写 `gorm:"column:xxx"`**。
- 软删除统一走 `gorm.io/plugin/soft_delete` 的 flag 模式：嵌入 `crud.BaseModel` 即自动获得（`IsDeleted soft_delete.DeletedAt`，`gorm:"softDelete:flag" json:"-"`）。插件在查询 / 更新 / 删除时自动追加 `is_deleted = 0`、`Delete()` 自动置 1，`Unscoped()` 可绕过。
  - **业务与 crud 基线不再手写 `is_deleted` 条件**；例外：`db.Table(...)` 的联表 / 子查询没有模型 schema，插件不生效，仍须显式写 `is_deleted = 0`。
  - 该字段底层是 uint，判空用 `crud.BaseModel` 提供的 `Deleted()`，写 `!x.IsDeleted` 无法编译。
  - 存量 PostgreSQL 库须先应用 `migrations/chaos_postgres_update.sql` 中 2026-10-04 那批 `boolean → smallint` 的 ALTER（插件以整数读写该列）；SQLite 以整数存储，无需变更。
- 新模型须在 `cmd/server/main.go` 的 `config.AutoMigrate(...)` 登记。
- `sql/chaos_postgres_schema.sql` 为手动导出基准快照，**AI 不得修改**；模型变动追加 `sql/chaos_postgres_update.sql`（增量日志，注日期与用途）。

## 路由

- `routes.SetupRouter(webFS fs.FS)` 注册，统一前缀 `/api`；`NoRoute` 回退 `index.html`。
- 前端新页面在 `src/router/index.ts` 的 `appRoutes` 追加；`meta.title/icon` 驱动菜单与面包屑（`hidden` 不进菜单，`fullscreen` 脱离布局）。
- 业务路由走业务包 `Register(api)`，**禁止在 `routes.go` 内联**。

## 后台任务

- 简单周期任务在业务包 `init()` 调 `scheduler.Register(name, interval, fn, enabledFn...)`；每任务独立 goroutine（含互斥 + panic 恢复）。
- 需要 cron 表达式 / 触发 HTTP、命令或系统动作的任务走 `internal/cronjob`（robfig/cron），不写进 `scheduler`。

## 前端通用

- 请求统一走 `src/utils/request.ts`，**不引入 axios**。
- 提示 / 确认 / 输入弹窗统一走 `src/utils/message.ts`（`showSuccess` / `showError` / `showWarning` / `showInfo` / `confirm` / `prompt`），**页面不直接 import ElMessage/ElMessageBox**。
- 写操作（二次确认 → 请求 → 提示 → 关弹窗 / 刷新）统一走 `src/composables/useCrudAction.ts` 的 `run(task, opts)`，**页面不重复 try/catch + toast + refresh 骨架**。
- 路由 history 模式（`createWebHistory`）。
- 主题 `src/theme.ts`（`dark` 默认 / `paper` 浅黄护眼），持久化 key `chaos-ui:theme`。
- 编辑器统一 Monaco。
- 资源接口门户：`src/api/<resource>.ts` 独占该资源类型与 api 函数，**禁止聚合进 `utils/api.ts`**。

## 业务模块脚手架基线

新增 / 重写业务模块一律套用本基线；参考实现：`internal/standarddata/` + `src/views/StandardData.vue` + `src/api/standardData.ts`。后端包内的 handler / model / service / repository 职责划分见下一节「分层契约」。

- **后端**：模型在 `internal/<模块>/<资源>.go`，CRUD 资源嵌入 `crud.BaseModel` 并走 `crud.Register[T]`（反射生成 list/get/create/update/delete 五路由，免写标准 handler）；实现 `TableName()` 与 `Register(rg *gin.RouterGroup)`，`routes.go` 只写 `<pkg>.Register(api)`。
  - `crud.Register` 返回该资源的路由组，扩展子路由直接挂在其上，**不重复写前缀字符串**。
  - 启停（PATCH `/:id/status`）一律走 `crud.RegisterToggle`，业务包只提供 `Setter` 与错误表。
  - 路径参数解析与领域错误映射一律走 `internal/httpx`（`ParseID` / `ParseParam` / `MapError` + `ErrRule` 表），**不再自写 `strconv.Atoi` 样板与 `writeXxxError` 的 switch**。
- **前端**：`src/api/<resource>.ts` 定义 `interface` + `useRestApi<Resource>('<resource>')`；`src/views/<Resource>.vue` 用通用 `DataTable`，仅声明 `columns`，无增删改查样板。
- **约定**：资源名前端 camelCase、表名 snake_case；时间列须 RFC3339 带时区 `value-format="YYYY-MM-DDTHH:mm:ssZ"`，否则 Go `time.Time` 反序列化报错。

> 存量聚合代码（旧模块）为历史遗留，不强制整改；被重写 / 扩展的模块须迁移到本基线。

## 分层契约（后端业务包）

`internal/<模块>/` 内按文件划分职责层（同包不同层），**依赖只能单向向下**：

`handler.go` → `service.go` → `repository.go` → `config.GetDB()`

`model.go` 可被任意层依赖，自身不依赖任何层。

| 文件 | 职责 | 允许 | 禁止 |
|---|---|---|---|
| `handler.go` | HTTP 适配：解析参数、调 service、组装响应、`Register` 挂载路由 | `gin.Context`、状态码映射、调用 service / repository 的只读查询 | 业务规则与分支编排、直接 `config.GetDB()`、直接文件 / 网络 IO |
| `service.go` | 用例编排与领域规则 | 调 repository、事务编排、跨模块调用、派生值计算 | `gin.Context`、`renv.Success/Error`、裸 SQL、HTTP 中间件 |
| `repository.go` | **包内唯一数据访问出口** | GORM 查询、软删过滤、分页 | 返回 DTO、业务判断、`gin.Context`、非数据访问的副作用 |
| `model.go` | 实体 + 领域不变量 | 结构体、`TableName()`、不碰 IO 的纯函数（校验 / 规范化 / 地址推导） | 任何 IO、引用 service 层全局状态（调度器 / 转发器 / 连接状态）、依赖 handler |
| `dto.go`（可选） | 请求 / 响应契约 | 具名 `XxxRequest` / `XxxResponse` 与 `toResponse` 组装 | 业务规则 |

硬规则：

1. **DB 单一入口**：业务包内 `config.GetDB()` 只允许出现在 `repository.go`（`internal/crud` 基线自身除外）。`init()` 不得播种数据或起后台 goroutine，启动副作用放 `seed.go` / `worker.go`，由 `cmd/server/main.go` 显式调用。
2. **model 不反向依赖**：需要运行状态（如"隧道是否在线"）时由调用方取好再传入，不在 model 里读全局运行时。
3. **repository 只进出模型**：返回 `[]Entity` / `*Entity`；DTO 转换在 handler 或 service 完成。
4. **handler 不承载业务**：出现 `if` 业务规则、循环编排、磁盘 / 网络副作用、事务控制，一律下沉 `service.go`（handler 只留参数解析 + 错误码映射 + 响应组装）。
5. **crud 回调只做转发**：`Opts[T]` 的 `Before/AfterXxx` 回调体只写一行转调 service，业务实现放 `service.go`；禁止在 `Register` 里内联闭包写业务逻辑。
6. **不在事务内做不可回滚的副作用**：建目录、删联接点等要么移到事务提交后，要么改成可补偿流程。
7. **平台实现 / 外部客户端 / 运行时各自独立成文件**：`xxx_windows.go` + `xxx_other.go`、`client.go`（SSH / MQTT / 上游 HTTP）、`scheduler.go` / `forwarder.go`（长驻运行时）、`seed.go`（默认数据）、`middleware.go`（ gin 中间件）。它们都**不是** `service.go`。
8. **优先具名 DTO**：响应优先用 `model.go` / `dto.go` 里的具名结构体；`gin.H` 仅限临时聚合视图。

参考实现：`internal/standarddata/`（handler 纯注册 + 一条自定义子路由、model 纯实体、repository 纯查询）。

> 存量模块为历史遗留，不强制整改；**被重写或扩展时必须迁移到本契约**。整改优先级：
> ① `envvar`（handler 兼任 service + repository）— **已完成**：拆出 `repository.go`（虚拟文件 / 快照，经 quickedit API）、`util.go`（私有纯工具），用例收进 `service.go`，handler 只剩解析与状态码映射。
> ② `filelink` — **已完成**：拆出 `repository.go`、`dto.go`，用例（`ValidateForCreate` / `CleanupLink` / `SetStatus`）收进 `service.go`，启停去掉事务改为「先副作用后落库 + 失败补偿」，回调改为 `BeforeCreate` 直接拒绝而非创建后回滚。
> ③ `project` — **已完成**：新建 `service.go`（回调体、`mergeUnclaimed`、`MoveProject`），repository 的 `Find*ByID` 转领域哨兵（`ErrProjectNotFound` / `ErrGroupNotFound` / `ErrInvalidPath`）。
> ④ `quickedit` — **已完成**：新建 `service.go`（登记 / 读写 / 保存 / 回滚 / 快照用例）、`storage.go`（磁盘与虚拟文件读写分派）、`dto.go`，handler 由 343 行降至 224 行。
> ⑤ `portfwd` — **已完成**：原 `service.go` 原地重命名为 `forwarder.go`（隧道长驻运行时，内容未改）；新建 `service.go`（校验 / 启停 / 测试连接 / 列表视图）、`client.go`（SSH 建连）、`dto.go`（响应契约）；`model.go` 只留实体与纯领域逻辑，`Find*ByID` 改 `int` 参数并转领域哨兵。
> ⑥ `cronjob` — **已完成**：原 `service.go` 拆为 `scheduler.go`（cron 运行时）+ `executor.go`（HTTP / Shell 动作）+ `seed.go`（默认任务）+ `service.go`（校验与用例）；新增 `dto.go`；`ExecuteJob` 改为返回本次运行记录，消除「执行后回查最新一条」的并发偏差。
> ⑦ `taskplan` — **已完成**：新增 `dto.go`（请求 / 响应契约）、`client.go`（原文代理拉取）；`service.go` 收编全部用例（计划增删改查 / 启停归档挂起 / 任务完成延期取消 / 复习与 AI 评分 / 三种统计），handler 三个文件只做解析与状态码映射；`BatchPostponeTasks` 的逐项结果不再丢弃。
> ⑧ 收尾四个模块 — **均已完成**：
>   - `mqttsync`：原 `service.go` 重命名为 `client.go`（MQTT 连接 / 加解密 / 发布订阅）；新建 `service.go`（`PrepareMessage` / `BroadcastMessage` / `Status` / `DeleteChannel` / `ListLatest`）与 `dto.go`；repository 只返回模型不再返回 DTO；删除 model 依赖 `NodeID()` 的 GORM 钩子（改为 service 显式补全）。
>   - `sdk`：新建 `seed.go`（默认类型，由 `app.go` 显式调用，取代 repository 的 `init` 播种）与 `dto.go`；`service.go` 收编校验与用例；修掉两处缺陷：`note` 的更新条件误用 `len(patch.Sources)`，以及更新后返回的是修改前的快照。
>   - `dbmonitor`：`postgres.go` / `sqlite.go` 改名为 `repo_postgres.go` / `repo_sqlite.go`；新建 `dto.go`；过滤 / 排序 / 分页切片由 handler 下沉到 `service.ListTableStats`。
>   - `apilog`：拆除 `service.go`，拆为 `middleware.go`（gin 中间件）与 `worker.go`（异步落库 + `Start()`）；后台 goroutine 改为由 `app.go` 显式启动，不再在 `init` 里起。

## 构建与部署

- 提交前 `go build ./...` 无错；后端逻辑改动后跑 `go test ./...`。
- 前端 `pnpm build` 产出 `dist/`；`scripts/chaos.deploy.ps1` 拷到 exe 同目录 `ui/`。后端**不内嵌前端**，启动时从 `CHAOS_UI_DIR` 或 `./ui` 读静态资源；前端改动只需重拷 `dist/`，无需重编二进制。
