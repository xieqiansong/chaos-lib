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
import "chaos-lib/config"

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

## 配置文件优先级

1. 环境变量 `APP_ENV` 决定加载哪个配置文件
2. 配置文件中的值覆盖默认值
3. 未配置的值使用 `setDefaults()` 中的默认值