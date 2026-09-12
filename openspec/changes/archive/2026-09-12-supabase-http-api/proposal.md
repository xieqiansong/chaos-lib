## Why

`chaos-lib` 的业务数据目前只落在本地（SQLite / 自建 PostgreSQL），没有云端读写通道。同时本机网络为 IPv4-only，而 Supabase 直连（`db.<ref>.supabase.co:5432`）在免费套餐下只提供 IPv6 出口，走 Postgres 线协议的直连路线不通；共享池虽然可用 IPv4，但要求引入连接池地址、用户名后缀、预编译语句开关等一整套约束。需要一条不依赖 Postgres 协议、在 IPv4 下即可工作的最小读写通道，先把链路打通，再决定哪些数据放上云。

## What Changes

- 新增 `chaos-go/internal/supabase` 包：基于标准库 `net/http` 的 Supabase Data API（PostgREST）客户端，提供 `Select` / `Insert` / `Upsert` / `Update` / `Delete` 五个方法
- 认证 MUST 只用 `apikey` 请求头；MUST NOT 把新式 API key（`sb_publishable_*` / `sb_secret_*`）放进 `Authorization: Bearer` —— 新式 key 不是 JWT，放入会被平台判为 `Invalid JWT`
- 后端读写 MUST 使用 `sb_secret_*`（secret key）：它绕过 RLS，无需为每张表编写策略；publishable key 受 RLS 约束，写入会被拒（`42501`）
- `config` 新增 `SupabaseConfig`（`SUPABASE_URL` / `SUPABASE_SECRET_KEY` / `SUPABASE_PUBLISHABLE_KEY` / `SUPABASE_SCHEMA` / `SUPABASE_TIMEOUT_SEC`），沿用既有 `-env=dev|prod` + `.env.{dev,prod}` 加载方式
- **不接 GORM**：本地 SQLite/GORM 体系（`config.GetDB()`、`AutoMigrate`、`chaos_postgres_update.sql`）保持不变，Supabase 只承载明确划归云端的数据；本变更不新增或修改任何 GORM 模型
- **不新增 HTTP 路由**：`routes.SetupRouter` / `routes.go` 不改动，Supabase 能力暂不对前端暴露，也不进入 `chaos-ui`
- 联通性验证走 Go 测试：单元测试用 `httptest.Server` 打桩（随 `go test ./...` 默认运行）；集成测试仅在设置了 `SUPABASE_SECRET_KEY` 与 `SUPABASE_DEMO_TABLE` 时真实请求 Supabase，否则 `t.Skip`
- `.env.example` 新增 Supabase 占位项；真实密钥只进 `.env.dev` / `.env.prod`（已被 `.gitignore` 忽略）

## Capabilities

### New Capabilities

- `supabase-data-api`: 通过 Supabase Data API（PostgREST）对云端表执行增删改查的能力，涵盖认证头规则、配置来源、表名约束、超时与错误语义

### Modified Capabilities

<!-- 无：不改变任何既有能力的功能规格 -->

## Impact

- 新增：`chaos-go/internal/supabase/client.go`、`chaos-go/internal/supabase/client_test.go`
- 修改：`chaos-go/config/env.go`（新增 `SupabaseConfig` 结构体、`AppConfig` 字段、`setConfigValue` 解析分支、`setDefaults` 默认值）
- 修改：`chaos-go/CONFIG.md`（补充 Supabase 配置项说明）
- 修改：`.env.example`（新增占位符）
- 不修改：`chaos-go/routes/routes.go`、`chaos-go/cmd/server/main.go` 的 `AutoMigrate` 登记、`chaos-go/sql/chaos_postgres_update.sql`、`chaos-ui`
- 新增依赖：无。仅使用标准库 `net/http` / `encoding/json` / `net/url`
- 前置条件：已取得 `sb_secret_*`；Supabase 侧存在一张用于验证的表（表名由 `SUPABASE_DEMO_TABLE` 指定）
- 安全约束：本仓库「公开 = 是」，`sb_secret_*` 与真实 Project URL MUST NOT 出现在任何被版本控制的文件中
