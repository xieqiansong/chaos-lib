# 模块索引（Module Index）

> 本文件是 chaos-lib 的「地图」，也是 **功能 → 后端模块 → 前端插件 → 路由 → API 前缀的唯一落点索引**；其他文档只引用本表，不重复维护清单。
> 分工：`README.md` 看全景与快速上手，`AGENTS.md` 是硬性规则权威，`docs/architecture.md` 只讲架构机制，配置字段以 `chaos-go/configs/config.example.yaml` 为准。
> 所有路径均按真实代码核对（2026-11-09）。约定之外的结构性偏离集中记录在 §5。
>
> **相关文档（同一 `docs/` 下）**：
> - `docs/architecture.md` — 系统拓扑、请求生命周期、插件机制、硬约束速记（一页纸心智模型）。
> - `docs/接口规范.md` — 早期 POST+Action 设计草稿，**不是当前契约**；实际信封与错误码以 `internal/framework/envelope`、`resp`、`httpx` 及 `chaos-ui/src/utils/request.ts` 为准。`chaos-ui/src/utils/request.ts` 等处注释仍沿用「仓库《接口规范.md》」的历史称呼，文件实际位于 `docs/`。

## 0. 怎么用这份索引

- 已知**功能名** → 查 §1 主表，得到后端/前端路径。
- 已知**后端包名**（如 `internal/domain/taskplan`）→ 直接开对应目录，按 `handler.go / service.go / repository.go / model.go / plugin.go / dto.go` 取文件（见 `AGENTS.md` 分层契约）。
- 已知**前端路由**（如 `/bookmarks`）→ 反查 §1 的「前端路由」列，或 `chaos-ui/src/router/index.ts`（`collectRoutes()` 汇总所有插件路由）。
- 已知**API 前缀** → 默认规则：模块名 kebab-case 即 `POST /api/v1/<prefix>/<action>`，实现见 `chaos-ui/src/utils/request.ts` 与各插件 `api.ts`。多资源模块会拆分前缀。

## 1. 功能 → 模块 → 路由 主表

| 功能 | 后端模块（路径） | 插件分类 | 前端插件（路径） | 前端路由 | API 前缀 | 备注 |
|------|------------------|----------|------------------|----------|----------|------|
| 任务管理（待办/cron/任务树） | `internal/domain/taskplan` | 业务 | `src/plugins/taskplan` | `/taskPlan` | `task-plan` | 含父子任务树 |
| 待办任务 | `internal/domain/taskplan` | 业务 | `src/plugins/taskplan` | `/pendingTask` | `task-plan` | 同模块不同页 |
| FSRS 间隔复习 | `internal/domain/taskplan`（`fsrs.go`） | 业务 | `src/plugins/taskplan` | `/taskPlan` | `task-plan` | AI 评分走 `platform/deepseek` |
| 定时任务（robfig/cron） | `internal/domain/cronjob` | 业务 | `src/plugins/cronjob` | `/cronJob` | `cron-job` | 独立模块，与 taskplan 无耦合 |
| 看板统计 | `internal/domain/dashboard` | 业务 | `src/plugins/dashboard` | `/dashboard` | `dashboard` | ECharts，前端辅助 `plugins/echarts` |
| 项目管理（含回收站） | `internal/domain/project` | 业务 | `src/plugins/project` | `/projectManage` | `project` | |
| 文件连接（Junction） | `internal/domain/filelink` | 业务 | `src/plugins/filelink` | `/fileLink` | `file-link` | 开关 `FEATURE_FILE_LINK` |
| 环境变量管理 | `internal/domain/envvar` | 业务 | `src/plugins/envvar` | `/environment` | `env-var` | TOML 快照 |
| 快捷编辑 | `internal/domain/quickedit` | 业务 | `src/plugins/quickedit` | `/quickEdit` | `quick-edit` | Monaco + 快照回滚 |
| SDK 版本切换 | `internal/domain/sdk` | 业务 | `src/plugins/sdk` | `/sdk` | `sdk` | 符号链接切换 |
| SSH 连接管理 | `internal/domain/sshconn`（仅登记元数据，接口随 `portfwd` 挂载） | 业务 | `src/plugins/sshconn` | `/sshConn` | `ssh-conn` | 凭据仅后端使用、响应脱敏 |
| SSH 端口转发 | `internal/domain/portfwd` | 业务 | `src/plugins/portfwd` | `/portForward` | `port-fwd` | 转发规则 |
| 数据监控（只读自省） | `internal/framework/dbmonitor` | 框架 | `src/plugins/dbmonitor` | `/databaseMonitor` | `db-monitor` | 双后端兼容 |
| 数据缓存 | `internal/framework/datacache` | 框架 | `src/plugins/datacache` | `/dataCache` | `data-cache` | 缓存条目 CRUD 与统计 |
| 数据字典（枚举中心） | `internal/domain/dict` | 业务 | `src/plugins/datadict` | `/dataDict` | `dict-types` / `dict-data` | **命名别名：dict ↔ datadict**（两个资源前缀） |
| 个人物品 * | `internal/domain/items` | 业务 | `src/plugins/items` | `/items` | `items` | 购入/补货/消耗/借还等；README 功能一览未逐条列出 |
| 标准数据 * | `internal/domain/standarddata` | 业务 | `src/plugins/standarddata` | `/standardData` | `standard-data` | README 功能一览未逐条列出 |
| 书签管理 | `internal/domain/proxy`（`bookmark.go`） | 业务 | `src/plugins/proxy` | `/bookmarks` | `bookmark` | 经浏览器扩展直连本机 |
| 浏览器历史 / 数据采集 | `internal/domain/proxy` | 业务 | `src/plugins/proxy` | `/browserHistory`（hidden） | `history` | 同一 proxy 模块 |
| 天气 / DeepSeek 余额代理 | `internal/domain/proxy` | 业务 | `src/plugins/proxy` | （藏在 proxy 页） | `proxy` | 百度天气 / 余额查询 |
| 格式转换 | `internal/domain/polyform` | 业务 | `src/plugins/polyform` | `/polyform` | `polyform` | polyform-tools |
| MQTT 多节点同步 | `internal/platform/mqttsync` | 平台（插件） | `src/plugins/mqttsync` | `/mqttSync` | `mqtt-sync` | AES 加密可配 |
| STUN 规则 / 端口转发同步 | `internal/platform/stunsync` / `internal/platform/stunpf` | 平台（插件） | （无） | （无） | （无） | 只声明内置 `Tasks`，不挂路由、不落 cronjob 表 |
| 通知推送（Toast/WxPusher） | `pkg/tools/alarm` | 工具 | （无） | （无） | （无） | 工具层，无独立模块/页面 |
| Supabase 数据通道 | `internal/platform/supabase` | 平台（适配器） | （无） | （无） | （无） | 仅后端客户端，非插件、无路由 |
| 系统周期任务 | `internal/domain/systemjob` | 业务 | （无） | （无） | （无） | 后台调度，无 UI |
| 旧地址兼容 | （无） | — | `src/plugins/taskplan` | `/task` → `/taskPlan` | （无） | hidden，仅重定向 |

> `*` = README 功能一览未逐条列出（已指向本表）。`hidden` = 不进侧边菜单但仍可访问。

## 2. 后端包分类（快速清点）

`chaos-go/` 下按「是否是插件」分三类，AI 找文件时先定类：

| 分类 | 目录 | 是否插件（`init()` 调 `plugin.Register`） | 数量 | 成员 |
|------|------|------------------------------------------|------|------|
| 业务模块 | `internal/domain/<x>` | 是 | 16 | taskplan, project, envvar, filelink, quickedit, proxy, portfwd, cronjob, items, dict, sdk, polyform, standarddata, systemjob, dashboard, sshconn |
| 框架基础设施 | `internal/framework/<x>` | 部分（dbmonitor, datacache 是插件，其余为基类/中间件） | 2 插件 + 14 基件 | web, router, routehub, plugin, config, crud, envelope, resp, httpx, middleware, pagination, scheduler, apilog, memcache, **dbmonitor**, **datacache** |
| 平台集成 | `internal/platform/<x>` | mqttsync / stunpf / stunsync（后两者只声明内置任务、不挂路由） | 3 插件 + 2 适配器 | **mqttsync**、**stunpf**、**stunsync**, deepseek, supabase |
| 宿主工具 | `pkg/tools` | 否 | 9 组（16 个 `.go`） | alarm, db, env_var, file_link, http, port_forwarding, project_fs, shell, uuid, util（部分含 `_windows`/`_other` 平台变体） |

> 注册插件合计 = 16（业务）+ 2（框架）+ 3（平台）= **21**（上表加粗者）。`internal/app/app.go` 用 blank import 拉起各插件 `init()`；`envvar`/`quickedit`/`apilog`/`mqttsync` 因需直接调用而用普通 import。

## 3. 前端结构（chaos-ui/src）

| 目录 | 作用 | 对应后端 |
|------|------|----------|
| `plugins/<name>/` | 业务插件：`api.ts`（接口门户）+ `<Name>.vue`（页面）+ `index.ts`（`registerPlugin` 登记 `routes`）；5 个存量插件未拆 `api.ts`（见 §5） | 与后端模块大体对应，非严格 1:1（见 §1，别名 dict↔datadict） |
| `plugins/echarts/` | 能力型插件（`routes: []`），仅 `install` 注册全局 ECharts，**无页面** | 前端辅助，无后端模块 |
| `plugins/index.ts` `registry.ts` `types.ts` | 插件登记中枢；`collectRoutes()` 汇总所有 `routes` 进 `router/index.ts` | — |
| `views/` | 应用级路由页：`Home.vue`( `/` )、`Example.vue`( `/example` hidden )、`MobileBoard.vue`( `/board` fullscreen )；另有 `Search.vue` + `search-engine-dropdown.ts` 为命令面板的全局搜索浮层，由 `App.vue` 直接引用，**不进路由表** | — |
| `components/` `composables/` `utils/` `router/` `theme.ts` | 通用组件（DataTable 等，含存量业务组件）、`useRestApi`/`useCrudAction`、`request`/`message`、路由汇总、主题 | — |

前端插件清单（19）：cronjob, dashboard, datacache, datadict, dbmonitor, echarts, envvar, filelink, items, mqttsync, polyform, portfwd, project, proxy, quickedit, sdk, sshconn, standarddata, taskplan。

## 4. 跨模块 / 外部边界（AI 易错点）

- **命名别名**：后端 `dict` ↔ 前端 `datadict`；API 前缀进一步拆为 `dict-types` / `dict-data`（两类资源）。
- **一对多**：`taskplan` 一个后端模块支撑 `/taskPlan` + `/pendingTask` 两个前端页；`proxy` 一个后端模块支撑 `/bookmarks` + `/browserHistory` + 天气/余额；`sshconn` / `portfwd` 各有独立前端页（`/sshConn` / `/portForward`），但后端 SSH 连接接口随 `portfwd` 挂载。
- **平台集成不全是「纯适配器」**：`mqttsync` / `stunpf` / `stunsync` 都有 `plugin.go`，其中 `stunpf` / `stunsync` 只声明内置周期任务、**不注册路由**；只有 `deepseek` / `supabase` 是无 `plugin.go` 的按需调用客户端。别在平台目录里找 `RegisterV1`。
- **通知没有独立模块**：`WxPusher` / Windows Toast 实现在 `pkg/tools/alarm.go`，由其他模块调用，无前端页面。
- **浏览器扩展**：`chaos-browser-extension/` 是独立可部署件，作为客户端直连后端 `proxy` 模块的 `/bookmarks`、`/history` 等接口；它本身不含业务后端逻辑。
- **软依赖示例**：`taskplan` 的 AI 评分依赖 `platform/deepseek`（运行时通过 `Meta.SoftDeps` 注入，非 import 硬依赖）。

## 5. 已知偏离约定（待收敛）

以下为代码仍偏离 `AGENTS.md` 的结构性事实，改动相关模块前先确认；文档之间已对齐，不再列为差异。

- **前端插件契约**：`polyform` / `proxy` / `quickedit` / `sdk` / `taskplan` 缺 `api.ts`，少量接口仍留在 `src/utils/api.ts`；`src/utils/polyform*.ts` 是业务逻辑却放在通用目录；`src/components/` 混入 `TaskPlanForm.vue`、`RatingDialog.vue` 等业务组件。
- **空壳后端插件**：`dashboard` / `polyform` / `sshconn` 只有 `plugin.go`，能力分别落在 `domain/taskplan`（统计）、前端格式转换工具、`domain/portfwd`（SSH 接口）。
- **插件注册位置**：21 个注册点中，`framework/dbmonitor` 与 `framework/datacache` 的 `plugin.Register` 写在 `handler.go`、没有 `plugin.go`，与 `AGENTS.md` 的约定不符；其余 19 个都在 `plugin.go`。
- **重复文件**：`chaos-ui/src/views/Search.vue` 与 `chaos-browser-extension/entrypoints/newtab/Search.vue`（及 `search-engine-dropdown.ts`）同源重复；`chaos-go/cmd/test/` 下被忽略的一次性脚本与 `cmd/tcp_over_websockets/` 功能相近。
- **命名不一致**：后端 `dict` ↔ 前端 `datadict`；`handler_v1.go`、`repo_postgres.go` 等文件命名变体；前端路由 camelCase（`/dataDict`、`/standardData`、`/taskPlan`）与后端全小写模块名不同。
- **历史草案**：`docs/接口规范.md` 是落地前的设计草稿，非当前契约；部分代码注释仍称其位于仓库根。
