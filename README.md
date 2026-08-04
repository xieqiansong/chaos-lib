# chaos-lib

一个自托管的个人工具箱，集成了任务管理、间隔复习（FSRS）、项目管理、环境变量管理、快捷编辑、端口转发等功能。

## 功能一览

- **📋 任务管理**：待办 / 定时（cron）/ 间隔复习三种类型，FSRS 算法驱动复习节奏，逾期检测，父子任务树
- **📖 FSRS 间隔复习**：内嵌 FSRS 算法，根据遗忘曲线自动计算下次最佳复习时间
- **📁 项目管理**：按目录组织本地项目（ProjectGroup / Project），记录 Git URL、访问时间，误删可回收站还原
- **🔗 文件连接**：管理 Windows 目录联接（Junction），创建 / 删除 / 切换 / 状态检测
- **⚙️ 环境变量管理**：读取系统 + 用户环境变量，增量修改（Set/Unset/Path），每次操作自动 TOML 快照
- **✏️ 快捷编辑**：登记文件 → 编辑保存（自动快照）→ 随时回滚到任意历史版本
- **📦 SDK 版本切换**：管理 JDK / Maven / Python / Llama 多版本，通过符号链接一键切换
- **🔌 端口转发**：本地端口 → 远程主机:端口，Web 界面管理启停
- **🔔 通知推送**：Windows 系统通知 + WxPusher 消息推送
- **📊 数据采集**：浏览器历史记录采集、DeepSeek 余额查询、百度天气代理

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
| 后端 | Go 1.26 + Gin + GORM |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus + ECharts |
| 数据库 | SQLite3（默认）/ PostgreSQL |
| 任务调度 | robfig/cron |

## 快速开始

### 准备工作

- Go 1.26+、pnpm（前端构建）

### 后端

```bash
cd chaos-go

# 复制配置模板（已内置 SQLite 默认值）
cp .env.example .env

# 运行（默认 SQLite，自动建表）
go run cmd/server/main.go -env=dev

# 或构建
go build -o chaos-go.exe cmd/server/main.go
```

### 前端构建并嵌入

后端通过 `//go:embed web` 内嵌前端静态文件。构建前端后，刷新二进制即可。

```bash
cd chaos-ui
pnpm install
pnpm build
```

## 配置

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `DB_TYPE` | 数据库类型：`sqlite` / `postgres` | `sqlite` |
| `DB_PATH` | SQLite 文件路径 | `data/chaos.db` |
| `DB_HOST` | PostgreSQL 主机 | `localhost` |
| `DB_PORT` | PostgreSQL 端口 | `5432` |
| `DB_USER` | PostgreSQL 用户 | `postgres` |
| `DB_PASSWORD` | PostgreSQL 密码 | - |
| `DB_NAME` | PostgreSQL 库名 | `chaos` |
| `SERVER_PORT` | HTTP 服务端口 | `8080` |
| `LOG_LEVEL` | 日志级别 | `info` |

详见 [`chaos-go/CONFIG.md`](chaos-go/CONFIG.md)。

## 项目结构

```
chaos-lib/
├── chaos-go/               # Go 后端（模块名 chaos-go）
│   ├── cmd/server/         # HTTP 服务入口（内嵌前端静态文件）
│   ├── internal/           # 业务逻辑（按功能分包，含数据模型 + handler）
│   ├── config/             # 配置加载、数据库连接、日志初始化
│   ├── routes/             # 路由定义
│   └── scheduler/          # 后台周期任务调度器
├── chaos-ui/               # Vue 3 + TypeScript 前端
└── scripts/                # 辅助脚本
```

## 参考

- [TypeWords](https://github.com/zyronon/TypeWords) — FSRS 间隔复习功能参考

## License

[MIT](LICENSE)
