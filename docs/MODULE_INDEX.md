# 模块索引（Module Index）

> 本文件是 chaos-lib 的「地图」，供 AI / 协作者快速定位 **功能 → 后端模块 → 前端插件 → 路由 → API 前缀**。
> 与 `README.md`（功能/结构叙述）、`AGENTS.md`（分层契约与硬规则）互补：README 看全景，AGENTS 看规范，**本文件看落点**。
> 所有路径均按真实代码核对（2026-10-25）。标 `*` 的是 README 功能表/页面表漏列、但代码里真实存在的模块。
>
> **相关文档（同一 `docs/` 下）**：
> - `docs/architecture.md` — 系统拓扑、请求生命周期、插件机制、硬约束速记（一页纸心智模型）。
> - `接口规范.md`（**根目录**）— POST+Action 信封、错误码、增删改查动作定义。保留在根目录是因为全仓 26 处代码注释都引用「仓库根《接口规范.md》」，移动会让这些引用失效。

## 0. 怎么用这份索引

- 已知**功能名** → 查 §1 主表，得到后端/前端路径。
- 已知**后端包名**（如 `internal/domain/taskplan`）→ 直接开对应目录，按 `handler.go / service.go / repository.go / model.go / plugin.go / dto.go` 取文件（见 `AGENTS.md` 分层契约）。
- 已知**前端路由**（如 `/bookmarks`）→ 反查 §1 的「前端路由」列，或 `chaos-ui/src/router/index.ts`（`collectRoutes()` 汇总所有插件路由）。
- 已知**API 前缀** → 默认规则：模块名 kebab-case 即 `POST /api/v1/<prefix>/<action>`（见 `接口规范.md`）。多资源模块会拆分前缀。

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
| SSH 连接管理 | `internal/domain/sshconn` | 业务 | `src/plugins/portfwd` | `/portForward` | `ssh-conn` | 前端并入 portfwd 页 |
| SSH 端口转发 | `internal/domain/portfwd` | 业务 | `src/plugins/portfwd` | `/portForward` | `port-fwd` | 转发规则 |
| 数据监控（只读自省） | `internal/framework/dbmonitor` | 框架 | `src/plugins/dbmonitor` | `/databaseMonitor` | `db-monitor` | 双后端兼容 |
| 数据缓存 * | `internal/framework/datacache` | 框架 | `src/plugins/datacache` | `/dataCache` | `data-cache` | README 页面表漏列 |
| 数据字典（枚举中心） | `internal/domain/dict` | 业务 | `src/plugins/datadict` | `/dataDict` | `dict-types` / `dict-data` | **命名别名：dict ↔ datadict**（两个资源前缀） |
| 个人物品 * | `internal/domain/items` | 业务 | `src/plugins/items` | `/items` | `items` | 购入/补货/消耗/借还等；README 功能表漏列 |
| 标准数据 * | `internal/domain/standarddata` | 业务 | `src/plugins/standarddata` | `/standardData` | `standard-data` | README 功能表漏列 |
| 书签管理 | `internal/domain/proxy`（`bookmark.go`） | 业务 | `src/plugins/proxy` | `/bookmarks` | `bookmark` | 经浏览器扩展直连本机 |
| 浏览器历史 / 数据采集 | `internal/domain/proxy` | 业务 | `src/plugins/proxy` | `/browserHistory`（hidden） | `history` | 同一 proxy 模块 |
| 天气 / DeepSeek 余额代理 | `internal/domain/proxy` | 业务 | `src/plugins/proxy` | （藏在 proxy 页） | `proxy` | 百度天气 / 余额查询 |
| 格式转换 | `internal/domain/polyform` | 业务 | `src/plugins/polyform` | `/polyform` | `polyform` | polyform-tools |
| MQTT 多节点同步 | `internal/platform/mqttsync` | 平台（插件） | `src/plugins/mqttsync` | `/mqttSync` | `mqtt-sync` | AES 加密可配 |
| 通知推送（Toast/WxPusher） | `pkg/tools/alarm` | 工具 | （无） | （无） | （无） | 工具层，无独立模块/页面 |
| Supabase 数据通道 | `internal/platform/supabase` | 平台（适配器） | （无） | （无） | （无） | 仅后端客户端，非插件、无路由 |
| 系统周期任务 | `internal/domain/systemjob` | 业务 | （无） | （无） | （无） | 后台调度，无 UI |

> `*` = README 未列出但代码真实存在。`hidden` = 不进侧边菜单但仍可访问。

## 2. 后端包分类（快速清点）

`chaos-go/` 下按「是否是插件」分三类，AI 找文件时先定类：

| 分类 | 目录 | 是否插件（有 `plugin.go` + `init` 注册） | 数量 | 成员 |
|------|------|------------------------------------------|------|------|
| 业务模块 | `internal/domain/<x>` | 是 | 16 | taskplan, project, envvar, filelink, quickedit, proxy, portfwd, cronjob, items, dict, sdk, polyform, standarddata, systemjob, dashboard, sshconn |
| 框架基础设施 | `internal/framework/<x>` | 部分（dbmonitor, datacache 是插件，其余为基类/中间件） | 2 插件 + 15 基件 | web, router, routehub, plugin, config, crud, envelope, resp, httpx, middleware, pagination, scheduler, apilog, security, memcache, **dbmonitor\***, **datacache\*** |
| 平台适配器 | `internal/platform/<x>` | 仅 mqttsync | 1 插件 + 4 适配器 | **mqttsync\***（插件）, deepseek, stunpf, stunsync, supabase（适配器，按需调用，不注册路由） |
| 宿主工具 | `pkg/tools` | 否 | 9 | alarm, db, env_var, file_link, http, port_forwarding, project_fs, shell, uuid |

> 插件合计 = 16（业务）+ 2（框架）+ 1（平台）= **19**，与 `AGENTS.md` 一致。

## 3. 前端结构（chaos-ui/src）

| 目录 | 作用 | 对应后端 |
|------|------|----------|
| `plugins/<name>/` | 业务插件：`api.ts`（接口门户）+ `<Name>.vue`（页面）+ `index.ts`（`registerPlugin` 登记 `routes`） | 与后端模块 1:1（见 §1，别名 dict↔datadict） |
| `plugins/echarts/` | 能力型插件（`routes: []`），仅 `install` 注册全局 ECharts，**无页面** | 前端辅助，无后端模块 |
| `plugins/index.ts` `registry.ts` `types.ts` | 插件登记中枢；`collectRoutes()` 汇总所有 `routes` 进 `router/index.ts` | — |
| `views/` | 应用级页面：`Home.vue`( `/` )、`Example.vue`( `/example` hidden )、`MobileBoard.vue`( `/board` fullscreen ) | — |
| `components/` `composables/` `utils/` `router/` `theme.ts` | 通用组件（DataTable 等）、`useRestApi`/`useCrudAction`、`request`/`message`、路由汇总、主题 | — |

前端插件清单（19）：cronjob, dashboard, datacache, datadict, dbmonitor, echarts, envvar, filelink, items, mqttsync, polyform, portfwd, project, proxy, quickedit, sdk, sshconn, standarddata, taskplan。

## 4. 跨模块 / 外部边界（AI 易错点）

- **命名别名**：后端 `dict` ↔ 前端 `datadict`；API 前缀进一步拆为 `dict-types` / `dict-data`（两类资源）。
- **一对多**：`taskplan` 一个后端模块支撑 `/taskPlan` + `/pendingTask` 两个前端页；`proxy` 一个后端模块支撑 `/bookmarks` + `/browserHistory` + 天气/余额；`sshconn` + `portfwd` 两个后端模块共用前端 `/portForward` 一页。
- **平台适配器 ≠ 插件**：`deepseek` / `stunpf` / `stunsync` / `supabase` 是 `internal/platform` 下的按需调用客户端，**没有 `plugin.go`、不注册路由**；只有 `mqttsync` 是注册插件。别在这四个目录里找 `RegisterV1`。
- **通知没有独立模块**：`WxPusher` / Windows Toast 实现在 `pkg/tools/alarm.go`，由其他模块调用，无前端页面。
- **浏览器扩展**：`chaos-browser-extension/` 是独立可部署件，作为客户端直连后端 `proxy` 模块的 `/bookmarks`、`/history` 等接口；它本身不含业务后端逻辑。
- **软依赖示例**：`taskplan` 的 AI 评分依赖 `platform/deepseek`（运行时通过 `Meta.SoftDeps` 注入，非 import 硬依赖）。

## 5. 已知文档与代码差异（供修正参考）

- README「功能一览」缺：**个人物品(items)**、**标准数据(standarddata)**；README「前端页面表」缺：**数据缓存(/dataCache)**。
- README「功能一览」的「通知推送」「Supabase 数据通道」「书签管理」在结构树上无同名后端模块，实际分别落在 `pkg/tools/alarm`、`platform/supabase`、`domain/proxy`。
- 前端页面表路由名（`/dataDict`、`/standardData`）与后端模块名（`dict`、`standarddata`）不完全一致，已在本表对齐。
