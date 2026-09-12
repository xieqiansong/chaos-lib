# chaos-lib

自托管的个人工具箱：Go 后端（Gin + GORM）+ Vue 3 前端，覆盖任务与间隔复习、项目管理、环境变量、文件连接、快捷编辑、SDK 版本切换、SSH 端口转发等日常开发场景。前端构建后经 `//go:embed` 内嵌进单个可执行文件，默认使用 SQLite，零外部依赖即可启动。

## 功能一览

- **📋 任务管理**：待办 / 定时（cron）/ 间隔复习三种任务计划，父子任务树，优先级、延期、逾期检测与统计
- **📖 FSRS 间隔复习**：内嵌 FSRS 算法（`internal/taskplan/fsrs.go`），按遗忘曲线计算下次最佳复习时间，支持 AI 评分辅助
- **📊 看板**：任务概览、每日 / 活跃 / 贡献统计（ECharts）
- **📁 项目管理**：按 ProjectGroup / Project 组织本地项目，记录 Git URL 与访问时间；移动（同卷 rename / 跨卷 copy）、访问、删除入回收站并可还原
- **🔗 文件连接**：管理 Windows 目录联接（Junction），创建 / 删除 / 切换 / 状态检测（功能开关 `FEATURE_FILE_LINK`）
- **⚙️ 环境变量管理**：读取系统 + 用户环境变量，增量修改（Set / Unset / Path 增删改），每次操作写入 TOML 快照并可查阅历史
- **✏️ 快捷编辑**：登记文件 → 编辑保存（自动快照）→ 回滚到任意历史版本；内嵌 Monaco 编辑器（含 diff），环境变量以「虚拟文件」形式接入
- **📦 SDK 版本切换**：管理 JDK / Maven / Python / Llama 多版本来源，通过符号链接一键切换
- **🔌 SSH 端口转发**：SSH 连接管理（凭据仅后端使用、响应脱敏）+ local(`ssh -L`) / remote(`ssh -R`) 转发规则，Web 界面启停
- **🔔 通知推送**：Windows 系统 Toast + WxPusher 消息推送
- **📡 数据采集与代理**：浏览器历史记录采集、DeepSeek 余额查询、百度天气代理
- **🖥️ 界面能力**：终端风格布局、命令面板（Ctrl+K）、主题切换（dark / paper 护眼）、全屏手机看板（`/board`）
- **☁️ Supabase 数据通道**：基于 PostgREST 的云端读写客户端（表名白名单、凭据仅走 `apikey` 头、更新删除强制带过滤），不对外暴露 HTTP 路由

## 截图展示

<p align="center"><b>📊 看板</b></p>
<p align="center"><img src="docs/screenshots/dashboard.png" alt="看板" width="1080" /></p>

<p align="center"><b>📋 任务管理（含 FSRS 复习）</b></p>
<p align="center"><img src="docs/screenshots/tasks.png" alt="任务管理" width="1080" /></p>

<p align="center"><b>⚙️ 环境变量管理</b></p>
<p align="center"><img src="docs/screenshots/env-variables.png" alt="环境变量" width="1080" /></p>

<p align="center"><b>✏️ 快捷编辑</b></p>
<p align="center"><img src="docs/screenshots/quick-edit.png" alt="快速编辑" width="1080" /></p>

<p align="center"><b>🔗 文件连接</b></p>
<p align="center"><img src="docs/screenshots/file-links.png" alt="文件连接" width="1080" /></p>

<p align="center"><b>📁 项目管理</b></p>
<p align="center"><img src="docs/screenshots/project-manage.png" alt="项目管理" width="1080" /></p>

<p align="center"><b>📦 SDK 版本切换</b></p>
<p align="center"><img src="docs/screenshots/sdk.png" alt="SDK版本" width="1080" /></p>

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.26 + Gin 1.12 + GORM 1.31（gzip 压缩、内置 pprof） |
| 前端 | Vue 3 + TypeScript + Vite 5 + Element Plus + ECharts 6 + Monaco Editor |
| 数据库 | SQLite3（`glebarez/sqlite`，默认）/ PostgreSQL（`gorm.io/driver/postgres`） |
| 任务调度 | robfig/cron v3 + 内置 `scheduler` 周期调度器 |
| 其他 | 后端：`x/crypto/ssh`（端口转发）、`go-toml`（快照）、`x/sys`（Windows API）；前端：markdown-it + DOMPurify、date-fns |

## 快速开始

准备工作：Go 1.26+、pnpm。

### 后端

```bash
cd chaos-go

# 复制配置模板（已内置 SQLite 默认值，开箱即用）
cp .env.example .env

# 运行（默认 SQLite，自动建表）
go run cmd/server/main.go -env=dev

# 或构建
go build -o chaos-go.exe ./cmd/server
```

### 前端

```bash
cd chaos-ui
pnpm install
pnpm dev        # 开发模式，/api 代理到 http://localhost:8080
pnpm build      # 构建到 dist/
```

后端通过 `//go:embed web` 内嵌前端静态文件，内嵌目录为 `chaos-go/cmd/server/web`。发布流程：`pnpm build` 后将 `dist/` 内容拷入该目录，再重新编译后端二进制。

### 一键部署（Windows）

`scripts/chaos.deploy.ps1`：安装依赖并构建前端 → 拷贝 `chaos-ui/dist` 到 `chaos-go/cmd/server/web` → `go build` 输出二进制 → `nssm restart chaos` 重启服务。

## 配置

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `SERVER_PORT` | HTTP 服务端口 | `8080` |
| `SERVER_HOST` | HTTP 监听地址 | `0.0.0.0` |
| `DB_TYPE` | 数据库类型：`sqlite` / `postgres` | `sqlite` |
| `DB_PATH` | SQLite 文件路径 | `data/chaos.db` |
| `DB_HOST` | PostgreSQL 主机 | `localhost` |
| `DB_PORT` | PostgreSQL 端口 | `5432` |
| `DB_USER` | PostgreSQL 用户 | `postgres` |
| `DB_PASSWORD` | PostgreSQL 密码 | - |
| `DB_NAME` | PostgreSQL 库名 | `chaos` |
| `DB_SSLMODE` | PostgreSQL SSL 模式 | `disable` |
| `PPROF_ENABLED` | 是否启用 pprof | `false` |
| `PPROF_PORT` | pprof 端口 | `6060` |
| `PPROF_HOST` | pprof 地址 | `localhost` |
| `FEATURE_FILE_LINK` | 文件连接功能开关 | `true` |
| `LOG_LEVEL` | 日志级别 | `info` |
| `DEEPSEEK_API_KEY` | DeepSeek 密钥（余额查询 / AI 评分，可选） | - |
| `BAIDU_AK` | 百度地图 AK（手机看板天气，可选） | - |
| `SUPABASE_URL` | Supabase 项目地址 | - |
| `SUPABASE_SECRET_KEY` | 后端专用 key（绕过 RLS，禁止提交） | - |
| `SUPABASE_PUBLISHABLE_KEY` | 前端只读预留 key（后端不使用） | - |
| `SUPABASE_SCHEMA` | 目标 schema | `public` |
| `SUPABASE_TIMEOUT_SEC` | 单次请求超时（秒） | `15` |
| `SUPABASE_TABLES` | 允许访问的表名清单（逗号分隔，留空则该通道不可用） | - |

环境切换：`-env=dev|prod` 优先于 `APP_ENV`，默认 `dev`；加载顺序为 `.env` > `.env.{env}`。详见 [`chaos-go/CONFIG.md`](chaos-go/CONFIG.md)。

## 项目结构

```
chaos-lib/
├── chaos-go/                    # Go 后端（module chaos-go）
│   ├── cmd/server/              # HTTP 服务入口，//go:embed web 内嵌前端
│   │   └── web/                 # 前端构建产物（部署脚本生成）
│   ├── cmd/tcp_over_websockets/ # TCP over WebSocket 隧道（独立服务）
│   ├── cmd/test/                # 一次性脚本：批量导入任务计划 / 连通性探针
│   ├── internal/                # 业务逻辑，按功能分包（模型 + handler 同包）
│   │   ├── taskplan/            # 任务计划 + 任务 + FSRS
│   │   ├── project/             # 项目管理 + 回收站
│   │   ├── envvar/              # 环境变量 + TOML 快照
│   │   ├── filelink/            # Windows Junction
│   │   ├── quickedit/           # 文件快照 / 回滚
│   │   ├── proxy/               # SDK 切换 / 浏览器历史 / 天气 / 余额
│   │   ├── portfwd/             # SSH 连接 + 端口转发
│   │   ├── notify/              # Windows 通知 / WxPusher
│   │   ├── deepseek/            # DeepSeek Chat API 客户端
│   │   ├── supabase/            # Supabase PostgREST 客户端
│   │   └── pagination/          # 统一分页
│   ├── config/                  # 配置加载 / DB 单例 / 日志
│   ├── routes/                  # 路由注册 + SPA 回退
│   ├── scheduler/               # 后台周期任务调度器
│   ├── sql/                     # PostgreSQL 基准快照 + 增量变更日志
│   └── models/ tools/ tasks/    # 遗留实现（现役以 internal/ 为准）
├── chaos-ui/                    # Vue 3 + TypeScript 前端
│   └── src/                     # views / components / router / utils
├── openspec/                    # 规格驱动开发（specs / changes / archive）
├── scripts/chaos.deploy.ps1     # 一键构建 + 部署
└── docs/screenshots/            # 界面截图
```

## 前端页面

| 路由 | 页面 | 说明 |
|------|------|------|
| `/dashboard` | 看板 | 任务概览与统计 |
| `/task` | 任务管理 | 待办 / cron / 间隔复习，任务树与 FSRS 复习 |
| `/projectManage` | 项目管理 | 项目组与项目 |
| `/sdk` | SDK版本 | 版本切换与来源管理 |
| `/fileLink` | 文件连接 | Junction 管理 |
| `/portForward` | 端口转发 | SSH 连接与转发规则 |
| `/quickEdit` | 快速编辑 | 文件编辑与快照回滚 |
| `/environment` | 环境变量 | 系统 / 用户环境变量管理 |
| `/browserHistory` | 历史记录 | 浏览器历史（隐藏，Web 端仅占位） |
| `/example` | 测试例子 | 组件示例（隐藏） |
| `/board` | 全屏看板 | 手机端时钟 + 天气（隐藏、全屏） |

路由表 `chaos-ui/src/router/index.ts` 的 `meta.title` / `meta.icon` 是侧边菜单与面包屑的唯一数据源，新增页面只需追加路由。

## 规格驱动开发

功能规格与变更规划由 [`openspec/`](openspec/) 管理：`openspec/specs/<module>/spec.md` 为真值源，变更提案放 `openspec/changes/<name>/`。使用方式见 [`openspec/README.md`](openspec/README.md)。

## 参考

- [TypeWords](https://github.com/zyronon/TypeWords) — FSRS 间隔复习功能参考
- [FSRS](https://github.com/open-spaced-repetition/free-spaced-repetition-scheduler) — 间隔复习算法

## License

[MIT](LICENSE)
