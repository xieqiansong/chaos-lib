# 架构概览（Architecture）

> 给 AI / 协作者的一页纸心智模型：系统怎么连、请求怎么走、插件怎么转、代码怎么分层。
> **事实源分工**：`AGENTS.md` 是硬性规则权威，`docs/MODULE_INDEX.md` 是模块/路由/前缀的唯一落点索引，`chaos-go/configs/config.example.yaml` 是配置字段权威；本文件只讲机制，不维护模块清单。`docs/接口规范.md` 是早期设计草案，不是当前契约。
> 本文件与代码核对（2026-11-09）；与代码不一致时以代码为准，并回改本文件。

## 1. 系统拓扑

```
┌─────────────────────┐   HTTP /api/v1/*   ┌──────────────────────────┐
│  chaos-ui (前端)     │ ─────────────────▶ │  chaos-go (后端, 单 exe)  │
│  Vue3+Vite, 静态资源 │                     │  Go 自研 web 框架+GORM   │
└─────────────────────┘                     └────────────┬─────────────┘
                                                         │ GORM
                                                         ▼
                              PostgreSQL(默认) ◀──────▶ SQLite(可切)
┌─────────────────────┐  直连本机 /api/v1   ┌──────────┴───────────────┐
│ chaos-browser-      │ ─────────────────▶ │  proxy 模块(bookmark/    │
│ extension (扩展)    │                     │  history) 等接口         │
└─────────────────────┘                     └──────────────────────────┘
```

- **三个可独立部署件**：`chaos-go`（后端单二进制）、`chaos-ui`（前端静态资源）、`chaos-browser-extension`（浏览器扩展，作为客户端直连本机后端）。
- 后端**不内嵌**前端：启动从 `CHAOS_UI_DIR` 或 exe 同目录 `ui/` 读静态资源，解析为 `webFS`（`cmd/server/main.go` 的 `resolveUIFS`）。前端改动只需重拷 `dist/`，无需重编后端。
- 数据库双后端：PostgreSQL（默认）/ SQLite3，业务代码经 GORM 透明切换。

## 2. 请求生命周期（POST + Action）

- 统一 `POST /api/v1/{prefix}/{action}`；HTTP 状态码恒 200，业务结果走 `envelope`（`code`/`message`/`data`/`requestId`）。契约以实现为准：`internal/framework/envelope`、`resp`、`httpx` 与 `chaos-ui/src/utils/request.ts`；`docs/接口规范.md` 是落地前的设计草案，仅作背景参考。
- 中间件链（`internal/framework/router/router.go` 装配）：`apilog` → `envelope.Middleware()` → `AuthMiddleware()`。
- 路由挂载：`routehub.MountAllV1`（存量 v1 路由） + `plugin.MountAll`（插件路由），统一前缀 `/api/v1`；`NoRoute` 回退 `index.html`。
- 分层调用（依赖单向向下）：`handler.go` → `service.go` → `repository.go` → `config.GetDB()`；`model.go` 不反向依赖任何层。
- 软删除：统一嵌入 `crud.BaseModel`，判空用 `Deleted()`（`!x.IsDeleted` 无法编译）；仅 `db.Table(...)` 的联表 / 子查询需手写 `is_deleted = 0`。存量 PostgreSQL 须先应用 `chaos_postgres_update.sql` 里的 `is_deleted` 类型 ALTER。

## 3. 插件系统（核心机制）

- 每个业务模块 = 一个**插件**。`<name>/plugin.go` 的 `init()` 调 `plugin.Register(Registration{ Meta, Models, RegisterRoutes, Tasks, Start, Stop })`。
- 触发：`internal/app/app.go` 顶部用 blank import `_ "chaos-go/internal/domain/<name>"` 拉起各插件 `init()`。
- 编排：`plugin.Build` 做拓扑排序 + 缺失/环检测（fail-fast）；硬依赖声明 `Dependencies`，循环依赖用 `Meta.SoftDeps`（接口型，运行时包级函数变量注入解环，如 `envvar↔quickedit`）。
- 插件系统：`Registration{ Meta, Models, RegisterRoutes, Tasks, Start, Stop }`。
- 启动顺序（`app.Run`）：config → DB → blank import 各插件 → `plugin.Build` → `config.AutoMigrate`（模型在 `plugin.go` 的 `Models` 登记）→ `plugin.StartAll` → `scheduler.StartBuiltin(plugin.TasksOf(regs))` → `router.SetupRouter`。
- 后台任务：**内置任务**由插件在 `Tasks` 中声明（`scheduler.TaskSpec`：Name / Cron / Timeout / SkipIfBusy / Run），由 `framework/scheduler` 统一运行，不落 `cron_jobs` 表，前端定时任务页只读展示；**用户任务**（HTTP / 命令 / 系统动作，可增删改启停）走 `domain/cronjob`（robfig/cron，秒级 `SecondOptional`）。两者共用同一 cron 解析器配置。

## 4. 后端目录拓扑（速记）

| 路径 | 角色 | 是否插件 |
|------|------|----------|
| `internal/domain/<x>` | 业务模块（16）；`dashboard`/`polyform`/`sshconn` 目前仅 `plugin.go` 登记元数据，实现散在别处 | 是 |
| `internal/framework/<x>` | 自研 web 框架 + 跨切面基件（16）；`dbmonitor`/`datacache` 注册为插件 | 2 个是 |
| `internal/platform/<x>` | 外部集成（5）：`mqttsync`/`stunpf`/`stunsync` 注册为插件（后两者只声明内置周期任务，不挂路由）；`deepseek`/`supabase` 为按需调用的纯客户端 | 3 个是 |
| `pkg/tools` | 宿主交互工具（9 组、16 个 `.go`，部分带 `_windows`/`_other` 平台变体），非业务；与 `internal/` 平级 | 否 |
| `cmd/server` `cmd/migrate` | HTTP 入口（解析 webFS，调 `app.Run`）/ 迁移命令行入口 | — |
| `configs` `migrations` `deployments` `api` | 配置模板与运行时（后者忽略）/ SQL 基准与增量 / 部署 / openapi 占位 | — |

> 注册插件合计 **21**（16 业务 + 2 框架 + 3 平台）。模块清单、路由与 API 前缀见 `docs/MODULE_INDEX.md` §1–§2，本表不重复枚举。

## 5. 前端架构（chaos-ui/src）

- 独立 pnpm 工程（自带依赖锁与 Vite 配置）；**插件化**：`src/plugins/<name>/` = `api.ts`（接口门户，`useRestApi`+`action`）+ `<Name>.vue`（页面，多用通用 `DataTable`）+ `index.ts`（`registerPlugin` 登记 `routes`/`meta`）。
- `src/plugins/registry.ts` 的 `collectRoutes()` 汇总所有插件 `routes` 进 `src/router/index.ts`；`meta.title`/`meta.icon` 驱动侧边菜单与面包屑（`hidden` 不进菜单，`fullscreen` 脱离布局）。`src/views/Search.vue` 是命令面板的全局搜索浮层，由 `App.vue` 直接引用、不进路由表。
- 请求统一 `src/utils/request.ts`（`action(prefix, name, data)`，不引 axios）；提示/确认走 `src/utils/message.ts`；写操作骨架走 `src/composables/useCrudAction.ts`。
- 编辑器统一 Monaco；主题 `src/theme.ts`（`dark` 默认 / `paper` 护眼）。
- 前端插件（19）与后端注册插件（21）**并非严格 1:1**（别名 `dict` ↔ `datadict`，另有一对多、一对零），对照表见 `MODULE_INDEX.md` §1。
- 存量偏离约定：`polyform`/`proxy`/`quickedit`/`sdk`/`taskplan` 尚未拆出 `api.ts`，少量接口仍留在 `src/utils/api.ts`；`src/utils/polyform*.ts` 与若干业务组件仍在通用目录。清单见 `MODULE_INDEX.md` §5。

## 6. 配置 / 部署 / 数据 指针

- 配置：`chaos-go/configs/config.yaml`（运行时，`.gitignore` 忽略）+ `config.example.yaml` 模板；覆盖项见 `chaos-go/CONFIG.md`。
- 部署：`scripts/chaos.deploy.ps1`（Windows 一键构建+拷产物+`nssm restart`）；`scripts/termux.build.chaos.sh`（Termux/Android 构建）。
- 数据：模型变动追加 `migrations/chaos_postgres_update.sql`（增量，注日期用途）；基准快照 `chaos_postgres_schema.sql` 与 `chaos-go/api/openapi.yaml` 为**冻结/占位**，AI 勿改。

## 7. 给 AI 的硬约束速记

1. `config.GetDB()` 只允许出现在 `repository.go`（framework 基线自身除外）；`init()` 不起 goroutine/不播种。
2. `handler` 只做参数解析 + 错误码映射 + 响应组装；业务规则/循环编排/磁盘网络副作用下沉 `service.go`。
3. `crud.RegisterActions` 的 `Before/After` 回调只写一行转调 service，禁止内联业务逻辑。
4. 不在事务内做不可回滚副作用（建目录/删联接点 → 移到提交后或可补偿）。
5. 时间列须 RFC3339 带时区（`YYYY-MM-DDTHH:mm:ssZ`），否则 Go `time.Time` 反序列化报错。
6. `platform/stunpf` 与 `platform/stunsync` 有 `plugin.go`，但只声明内置周期任务、**不注册路由**；`deepseek`/`supabase` 才是无 `plugin.go` 的纯客户端。别在平台目录里找 `RegisterV1`。

> 以上规则以 `AGENTS.md` 为权威，本表仅作速记；若冲突以代码与 `AGENTS.md` 为准。
