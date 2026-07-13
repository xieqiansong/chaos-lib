# chaos-lib 项目规则

> 本文件为项目唯一规则来源（AGENTS.md），供所有 AI 编程助手统一遵循。
> 已对齐 `chaos-go/`（Go 后端）与 `chaos-ui/`（Vue3 前端）真实结构。

## 项目概述

仓库根包含两大部分与一个共享 SQL 目录：

- `chaos-go/`：Go 1.26.3 后端，**模块名为 `chaos-lib`**，使用 Gin 框架 + GORM ORM，PostgreSQL 数据库。
- `chaos-ui/`：Vue 3 + TypeScript 前端（Element Plus、ECharts、Monaco、Vite，包管理用 pnpm）。
- `sql/`：DDL 脚本（`chaos_ddl.sql`、`chaos.sql`）。

后端功能：知识卡片（FSRS 算法，实现位于 `tools/fsrs.go`，作用于 `TaskPlan` 模型）、浏览器历史记录、
SDK 版本切换（`services/sdk.go`）、文件连接管理、端口转发、环境变量管理、任务计划（`TaskPlan`/`Task`）、
快捷编辑、通知（wxpusher）等，均带 `FEATURE_*` 功能开关。

## 项目结构

```
chaos-lib/
├── chaos-go/                 # Go 后端（模块名 chaos-lib）
│   ├── cmd/
│   │   ├── server/               # 主 HTTP 服务入口 main.go
│   │   ├── tcp_over_websockets/  # 独立 CLI 工具（TCP over WebSocket）
│   │   └── test/                 # 测试 / 临时入口
│   ├── config/                   # 配置与数据库初始化
│   │   ├── config.go             # 配置加载、GetDB()、GetConfig()
│   │   └── env.go                # 环境变量结构体与解析
│   ├── models/                   # 数据模型定义（GORM 标签）
│   │   ├── browser_history.go
│   │   ├── env_variable.go
│   │   ├── file_link.go
│   │   ├── port_forwarding.go
│   │   ├── quick_edit.go
│   │   ├── task_plan.go          # 含 FSRS 字段
│   │   └── task.go
│   ├── routes/                   # HTTP 路由定义
│   │   └── routes.go
│   ├── services/                 # HTTP handlers 与业务逻辑
│   │   ├── balance.go
│   │   ├── browser_history.go
│   │   ├── env_variable.go
│   │   ├── file_link.go
│   │   ├── notify.go
│   │   ├── port_forwarding.go
│   │   ├── quick_edit.go
│   │   ├── sdk.go
│   │   ├── task_scheduler.go
│   │   └── task.go
│   ├── tools/                    # 可复用工具函数与类型
│   │   ├── env_toml.go
│   │   ├── environment.go
│   │   ├── environment_windows.go
│   │   ├── fsrs.go               # FSRS 算法实现
│   │   ├── port_forwarding.go
│   │   ├── win_notify.go
│   │   └── wxpusher.go
│   ├── tasks/                    # 后台任务调度
│   │   └── tasks.go
│   ├── go.mod
│   └── CONFIG.md                 # 配置管理说明
├── chaos-ui/                     # Vue3 + TS 前端
├── sql/                          # SQL 脚本（chaos_ddl.sql, chaos.sql）
├── scripts/                      # 辅助脚本
└── data/
```

## 代码规范（Go 后端）

### 命名约定

- **包名**：小写、单数（`models`、`services`、`routes`、`config`、`tools`、`tasks`）
- **结构体**：PascalCase（`TaskPlan`、`BrowserHistory`、`PortForwarding`）
- **接口**：PascalCase，以 `er` 结尾（如适用）
- **常量**：PascalCase（`RatingAgain`、`StateNew`、`TaskPlanStatusCreated`）
- **变量**：camelCase（`targetHost`、`wsConn`、`activeConns`）
- **函数**：PascalCase（导出）或 camelCase（私有）
- **文件**：snake_case（`task_plan.go`、`tcp_over_websockets`）

### 模型标签约定

`models` 包下的 GORM 模型结构体遵循：

- **只在必要时添加标签**：不要为每个字段都写 `gorm:"column:xxx"`；需要时再显式指定。
- **主键**：`ID int \`gorm:"primaryKey"\``
- **默认值**：需要默认值时才加，如 `Status TaskPlanStatus \`gorm:"default:created"\``
- **时间戳**：创建/更新时间默认值 `CreatedAt time.Time \`gorm:"default:CURRENT_TIMESTAMP"\``
- **其他普通字段**：留空标签 ``（两个反引号之间为空）即可，GORM 会自动推断列名，JSON 序列化也正常。

示例（取自实际 `models/task_plan.go`，省略部分 FSRS 字段）：

```go
type TaskPlan struct {
    ID               int            `gorm:"primaryKey"`
    ParentID         *int           ``
    Name             string         ``
    Status           TaskPlanStatus `gorm:"default:created"`
    PlanType         TaskPlanType   `gorm:"default:todo"`
    CronExpr         *string        ``
    FsrsStability    float64        `gorm:"default:0"`
    FsrsDifficulty   float64        `gorm:"default:0"`
    FsrsLastReviewAt *time.Time     ``
    Remark           *string        ``
    CreatedAt        time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
    UpdatedAt        time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
}
```

### 包组织

- 每个包一个目录，相关功能放同一包
- `models/`：纯数据结构 + GORM 标签
- `services/`：HTTP handlers + 业务逻辑
- `tools/`：可复用的工具函数与类型（如 FSRS、端口转发、环境操作）
- `tasks/`：后台任务调度
- `cmd/`：应用入口点

### 导入规范

```go
import (
    // 标准库
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    // 项目内部包（模块名 chaos-lib）
    "chaos-lib/config"
    "chaos-lib/models"
    "chaos-lib/services"

    // 第三方库
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)
```

- 按 标准库 → 项目内部 → 第三方库 分组
- 每组按字母顺序排列，组间用空行分隔

### HTTP Handler 规范

```go
// 函数签名
func HandlerName(c *gin.Context) {
    // 1. 参数绑定与验证
    var req struct {
        Field string `binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 2. 业务逻辑
    // ...

    // 3. 返回响应
    c.JSON(http.StatusOK, result)
}
```

- Handler 放在 `services/` 包
- 使用 `c.ShouldBindJSON()` 进行参数绑定
- 错误返回 `gin.H{"error": "message"}`
- 使用标准 HTTP 状态码（200、201、400、404、500）

### 数据库访问

- 通过 `config.GetDB()` 获取数据库实例（定义于 `config/config.go`）
- 使用 GORM 进行 ORM 操作
- 模型定义使用 `gorm:"primaryKey"` 等标签
- 错误处理：检查 `.Error` 字段
- 软删除约定：模型含 `IsDeleted bool \`gorm:"default:false"\``，查询普遍带 `is_deleted = ?`

```go
// 查询示例
var cards []models.TaskPlan
if err := config.GetDB().Where("is_deleted = ?", false).Find(&cards).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}

// 创建示例
if err := config.GetDB().Create(&card).Error; err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

### 路由定义

- 所有路由在 `routes.SetupRouter()`（`routes/routes.go`）中定义
- 使用 RESTful 风格
- 相关路由使用路由组（`r.Group()`）
- 注释说明路由功能

```go
func SetupRouter() *gin.Engine {
    r := gin.Default()

    // 功能模块路由
    r.GET("/resource", services.GetResources)
    r.POST("/resource", services.CreateResource)

    // 使用路由组
    group := r.Group("/prefix")
    {
        group.POST("/", services.Create)
        group.GET("/:id", services.Get)
    }

    return r
}
```

### 错误处理

- 使用 `if err := ...; err != nil` 模式
- HTTP handler 中返回适当的 HTTP 状态码与错误消息
- CLI 工具中使用 `log.Fatalf()` 或 `log.Printf()`
- 不要忽略错误

### 并发模式

- 使用 `context.Context` 进行取消控制
- 使用 `sync.WaitGroup` 等待 goroutine 完成
- 使用 `sync.Mutex` 或 `sync.RWMutex` 保护共享状态
- 使用 `sync.Once` 确保一次性操作
- 后台任务统一由 `tasks/` 包调度（`tasks.go`）

```go
var wg sync.WaitGroup
wg.Add(2)

go func() {
    defer wg.Done()
    // 工作
}()

wg.Wait()
```

### 日志规范

- 使用标准库 `log` 包（`log.Printf` / `log.Println`）
- 服务器启动与关键状态使用 emoji 标识（项目既有风格，保持一致）：
  - `🌐 启动 HTTP 服务 ...`
  - `✅` 成功、`❌` 失败、`🔄` 进行中
- 关键操作记录日志（连接、断开、错误、配置加载）

### 配置管理

- 通过 `config/` 包管理（`config.GetConfig()`、`config.GetDB()`）
- 环境切换：命令行 `-env=dev|prod`，或环境变量 `APP_ENV`；默认 `dev`
- 配置文件位于 `chaos-go/` 根目录：
  - `.env.dev` / `.env.prod`（已在 `.gitignore`，**勿提交**）
  - `.env.example` 为提交到仓库的模板
- 新增配置项：在 `config/env.go` 结构体中加字段 → `setConfigValue()` 解析 → `setDefaults()` 默认值
- 功能开关（如 `FEATURE_KNOWLEDGE_CARD`、`FEATURE_FILE_LINK`）通过配置控制

## 技术栈

### 核心依赖（go.mod 已锁定版本）

- **Web 框架**：`github.com/gin-gonic/gin v1.12.0`
- **ORM**：`gorm.io/gorm v1.31.1`
- **数据库驱动**：`gorm.io/driver/postgres v1.6.0`（底层 `github.com/lib/pq`）
- **WebSocket**：`github.com/gorilla/websocket v1.5.3`
- **定时任务**：`github.com/robfig/cron/v3`
- **Go 版本**：1.26.3

### 数据库

- PostgreSQL
- 使用 GORM 自动迁移或手动 DDL（见 `sql/chaos_ddl.sql`）
- 主键使用 `INTEGER GENERATED BY DEFAULT AS IDENTITY`
- 时间字段使用 `TIMESTAMPTZ`
- 软删除约定：模型含 `IsDeleted bool \`gorm:"default:false"\``，查询普遍带 `is_deleted = ?`

### 前端技术栈（chaos-ui）

- Vue 3 + TypeScript + Vite + pnpm
- UI：Element Plus；图表：ECharts（vue-echarts）；编辑器：Monaco
- 路由：vue-router

## 开发工作流

### 添加新功能模块（Go）

1. 在 `models/` 中定义数据模型（按需加 GORM 标签）
2. 在 `services/` 中实现 HTTP handlers 与业务逻辑
3. 在 `routes/routes.go` 中注册路由
4. 如需数据库表，在 `sql/chaos_ddl.sql` 中添加 DDL

### 运行项目

```bash
# 启动 HTTP 服务（dev 环境）
go run chaos-go/cmd/server/main.go -env=dev

# 生产环境
go run chaos-go/cmd/server/main.go -env=prod
# 或构建后运行
.\chaos-go.exe -env=prod

# TCP over WebSocket 工具
go run chaos-go/cmd/tcp_over_websockets/server 7002
go run chaos-go/cmd/tcp_over_websockets/client 13306 ws://localhost:7002/websocket/forward/host/port
```

### 性能分析

```go
// main.go 中启用 pprof（受 PPROF_ENABLED 配置控制，默认端口 6060）
// 访问 http://localhost:6060/debug/pprof/
```

## 注意事项

- Windows 环境下创建符号链接需要管理员权限
- 注释掉的代码应说明原因或考虑删除
- GORM 标签约定见上方「模型标签约定」：仅在必要时加标签，普通字段留空标签即可（不要强行补 `column:`）
- 全局变量（如 `GlobalPortForwarder`）应谨慎使用
- 确保 `context` 正确传递与取消
- WebSocket 连接需要正确处理关闭与重连
- 切勿将 `.env.dev` / `.env.prod` 提交到版本库
