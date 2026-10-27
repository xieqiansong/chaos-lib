# 架构概览（Architecture）

> 给 AI / 协作者的一页纸心智模型：系统怎么连、请求怎么走、插件怎么转、代码怎么分层。
> 细节规范见 `AGENTS.md`（分层契约/硬规则）、`接口规范.md`（POST+Action 信封）、`MODULE_INDEX.md`（功能→模块落点）、`README.md`（全景）。
> 本文件与代码核对（2026-10-25）。

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

- 统一 `POST /api/v1/{prefix}/{action}`；HTTP 状态码恒 200，业务结果走 `envelope`（`code`/`message`/`data`/`requestId`）。完整信封与错误码见 `接口规范.md`。
- 中间件链（`internal/framework/router/router.go` 装配）：`apilog` → `envelope.Middleware()` → `AuthMiddleware()`。
- 路由挂载：`routehub.MountAllV1`（存量 v1 路由） + `plugin.MountAll`（插件路由），统一前缀 `/api/v1`；`NoRoute` 回退 `index.html`。
- 分层调用（依赖单向向下）：`handler.go` → `service.go` → `repository.go` → `config.GetDB()`；`model.go` 可被任意层依赖。
- 软删除：`crud.BaseModel` 嵌入后即获 `IsDeleted` flag 模式，查询/更新/删除自动追加条件；联表/子查询无 schema 时仍需手写 `is_deleted = 0`。

## 3. 插件系统（核心机制）

- 每个业务模块 = 一个**插件**。`<name>/plugin.go` 的 `init()` 调 `plugin.Register(Registration{ Meta, Models, RegisterRoutes, Start, Stop })`。
- 触发：`internal/app/app.go` 顶部用 blank import `_ "chaos-go/internal/domain/<name>"` 拉起各插件 `init()`。
- 编排：`plugin.Build` 做拓扑排序 + 缺失/环检测（fail-fast）；硬依赖声明 `Dependencies`，循环依赖用 `Meta.SoftDeps`（接口型，运行时包级函数变量注入解环，如 `envvar↔quickedit`）。
- 启动顺序（`app.Run`）：config → DB → blank import 各插件 → `plugin.Build` → `config.AutoMigrate`（模型在 `plugin.go` 的 `Models` 登记）→ `plugin.StartAll` → `router.SetupRouter`。
- 后台任务：**旧 `scheduler.Register` 已废弃**。统一走插件 `Start` 钩子起 goroutine（含互斥 + panic 恢复）；需 cron 表达式/触发 HTTP·命令·系统动作的任务走 `domain/cronjob`（robfig/cron，秒级 `SecondOptional`）。

## 4. 后端目录拓扑（速记）

| 路径 | 角色 | 是否插件 |
|------|------|----------|
| `internal/domain/<x>` | 业务模块（16） | 是 |
| `internal/framework/<x>` | 自研 web 框架 + 跨切面基件（15）；其中 `dbmonitor`/`datacache` 是插件 | 2 个是 |
| `internal/platform/<x>` | 外部集成适配器（deepseek/stunpf/stunsync/supabase 为按需调用，**无 plugin.go**；仅 `mqttsync` 是插件） | 1 个是 |
| `pkg/tools` | 宿主交互工具（alarm/db/env_var/file_link/http/port_forwarding/project_fs/shell/uuid），非业务 | 否 |
| `cmd/server` | HTTP 入口，解析 webFS，调 `app.Run` | — |
| `configs` `migrations` `deployments` `api` `scripts` `test` | 配置/SQL 基准与增量/部署/openapi 占位/脚本/测试 | — |

> 落点细节见 `docs/MODULE_INDEX.md`。

## 5. 前端架构（chaos-ui/src）

- pnpm workspace；**插件化**：`src/plugins/<name>/` = `api.ts`（接口门户，`useRestApi`+`action`）+ `<Name>.vue`（页面，多用通用 `DataTable`）+ `index.ts`（`registerPlugin` 登记 `routes`/`meta`）。
- `src/plugins/registry.ts` 的 `collectRoutes()` 汇总所有插件 `routes` 进 `src/router/index.ts`；`meta.title`/`meta.icon` 驱动侧边菜单与面包屑（`hidden` 不进菜单，`fullscreen` 脱离布局）。
- 请求统一 `src/utils/request.ts`（`action(prefix, name, data)`，不引 axios）；提示/确认走 `src/utils/message.ts`；写操作骨架走 `src/composables/useCrudAction.ts`。
- 编辑器统一 Monaco；主题 `src/theme.ts`（`dark` 默认 / `paper` 护眼）。
- 前端插件与后端模块 1:1（别名：`dict` ↔ `datadict`），详见 `MODULE_INDEX.md` §1。

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
6. 平台适配器（deepseek/stunpf/stunsync/supabase）按需调用，**没有 `plugin.go`、不注册路由**，别在这几个目录找 `RegisterV1`。

> 以上规则以 `AGENTS.md` 为权威，本表仅作速记；若冲突以代码与 `AGENTS.md` 为准。
