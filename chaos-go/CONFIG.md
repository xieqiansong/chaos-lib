# 配置管理说明

## 概述

项目使用基于环境变量的配置管理系统，支持开发和生产环境的配置分离。

## 环境切换

### 方式一：命令行参数（推荐）

```bash
# 开发环境
go run cmd/server/main.go -env=dev

# 生产环境
go run cmd/server/main.go -env=prod

# 构建后运行
.\chaos-go.exe -env=prod
```

### 方式二：环境变量

```bash
# Windows PowerShell
$env:APP_ENV="prod"
go run cmd/server/main.go

# Linux/Mac
APP_ENV=prod go run cmd/server/main.go
```

### 方式三：默认行为

- 未指定环境时，默认使用 `dev` 环境
- 系统会自动加载对应的 `.env.dev` 或 `.env.prod` 文件

## 配置文件

### 文件位置

配置文件位于项目根目录：

- `.env.dev` - 开发环境配置
- `.env.prod` - 生产环境配置

### 配置项说明

#### 服务器配置

- `SERVER_PORT` - HTTP 服务端口（默认：8080）
- `SERVER_HOST` - HTTP 服务地址（默认：0.0.0.0）

#### 数据库配置

- `DB_HOST` - 数据库主机地址
- `DB_PORT` - 数据库端口（默认：5432）
- `DB_USER` - 数据库用户名
- `DB_PASSWORD` - 数据库密码
- `DB_NAME` - 数据库名称
- `DB_SSLMODE` - SSL 模式（默认：disable）

#### Pprof 性能分析

- `PPROF_ENABLED` - 是否启用 pprof（true/false）
- `PPROF_PORT` - pprof 端口（默认：6060）
- `PPROF_HOST` - pprof 地址（默认：localhost）

#### 功能开关

- `FEATURE_FILE_LINK` - 文件连接功能（true/false）

#### 日志配置

- `LOG_LEVEL` - 日志级别（debug/info/warn/error）

## 开发环境配置示例

```env
# .env.dev
SERVER_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_PASSWORD=dev_password
PPROF_ENABLED=true
FEATURE_FILE_LINK=true
FEATURE_PORT_FORWARD=true
FEATURE_KNOWLEDGE_CARD=true
FEATURE_BROWSER_HISTORY=true
FEATURE_SDK_SWITCH=true
LOG_LEVEL=debug
```

## 生产环境配置示例

```env
# .env.prod
SERVER_PORT=8080
DB_HOST=ubuntu.lan
DB_PORT=30101
DB_PASSWORD=your_secure_password
PPROF_ENABLED=false
FEATURE_FILE_LINK=true
FEATURE_PORT_FORWARD=true
FEATURE_KNOWLEDGE_CARD=true
FEATURE_BROWSER_HISTORY=true
FEATURE_SDK_SWITCH=true
LOG_LEVEL=info
```

## 新增配置项

如需添加新的配置项：

1. 在 `config/env.go` 的对应配置结构体中添加字段
2. 在 `setConfigValue()` 函数中添加解析逻辑
3. 在 `setDefaults()` 函数中设置默认值
4. 更新 `.env.example` 文件

## 安全注意事项

- ⚠️ **不要**将 `.env.dev` 和 `.env.prod` 提交到版本库
- ✅ 这些文件已在 `.gitignore` 中配置
- ✅ 只提交 `.env.example` 作为配置模板
- ⚠️ 生产环境密码应使用强密码并定期更换

## 使用配置

在代码中获取配置：

```go
import "chaos-go/config"

// 获取完整配置
cfg := config.GetConfig()

// 获取数据库配置
dsn := cfg.Database.GetDSN()

// 获取服务器地址
addr := cfg.Server.GetAddress()

// 检查功能开关
if cfg.Features.EnableFileLink {
// 启用文件连接功能
}
```

## Supabase 云端数据通道（Data API / PostgREST）

用于把明确划归云端的数据读写到 Supabase，走纯 HTTPS（不依赖 Postgres 线协议，IPv4 网络即可工作）。
实现见 `chaos-go/internal/supabase`；**不接 HTTP 路由**，因此不会出现在 `/api` 下。

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `SUPABASE_URL` | 项目地址，形如 `https://<project-ref>.supabase.co` | - |
| `SUPABASE_SECRET_KEY` | `sb_secret_*`，后端专用，绕过 RLS | - |
| `SUPABASE_PUBLISHABLE_KEY` | `sb_publishable_*`，受 RLS 约束；后端**不使用**，预留给将来的前端只读场景 | - |
| `SUPABASE_SCHEMA` | 目标 schema | `public` |
| `SUPABASE_TIMEOUT_SEC` | 单次请求超时（秒） | `15` |
| `SUPABASE_TABLES` | 允许访问的表名清单，逗号分隔（空项自动丢弃） | - |

**可用性判定**：`SUPABASE_URL`、`SUPABASE_SECRET_KEY`、`SUPABASE_TABLES` 三者同时非空，该通道才被视为可用；否则 `supabase.Get()` 返回 `nil`，调用方必须判空。

### 注意事项

- **凭据只放 `apikey` 请求头**。Supabase 新式凭据（`sb_publishable_*` / `sb_secret_*`）不是 JWT，放进 `Authorization: Bearer` 会被平台判为 `Invalid JWT`。
- **后端读写必须用 secret key**。publishable key 等价于旧的 `anon`，受行级安全性（RLS）约束：RLS 启用且无策略时读取恒为空集、写入被拒（`42501`）。secret key 具备 `bypassrls`，只要请求不携带用户 access token 就绕过 RLS，无需为每张表编写策略。
- **表名白名单**：不在 `SUPABASE_TABLES` 中的表名会在发出请求前被拒绝，不会产生任何外网请求。
- **更新与删除必须带过滤条件**：`Update` / `Delete` 未携带过滤条件时直接返回错误，避免整表被改写或清空。
- **非 `public` schema**：除在 `SUPABASE_SCHEMA` 指定外，还必须先在该项目的 Dashboard → API Settings 中把该 schema 加入 **Exposed schemas**，否则请求不会生效。
- **安全**：`SUPABASE_SECRET_KEY` 只写在 `.env` / `.env.dev` / `.env.prod`（均已被 `.gitignore` 忽略）。本仓库对外公开，禁止把 secret key 与真实 Project URL 写入任何被版本控制的文件。
- **免费套餐项目会休眠**：冷启动时首个请求可能较慢或失败，此时由调用方重试；客户端不做自动重试（写操作重试有重复写入风险）。

## 配置文件优先级

1. 环境变量 `APP_ENV` 决定加载哪个配置文件
2. 配置文件中的值覆盖默认值
3. 未配置的值使用 `setDefaults()` 中的默认值