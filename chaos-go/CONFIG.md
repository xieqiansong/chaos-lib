# 配置管理说明

## 概述

项目使用基于环境变量的配置管理系统，统一从项目根目录的 `.env` 文件加载配置。

## 配置文件

### 文件位置

配置文件位于项目根目录：

- `.env` - 唯一配置文件，程序启动时默认加载（缺失则使用内置默认值）

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
# .env
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
# .env
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

1. 在 `config/env.go` 的对应配置结构体中添加字段，并写好 `env:"KEY"` 与 `envDefault:"..."` tag
2. 更新 `.env.example` 文件

## 安全注意事项

- ⚠️ **不要**将 `.env` 提交到版本库
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
if cfg.FeatureFileLink {
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
- **安全**：`SUPABASE_SECRET_KEY` 只写在 `.env`（已被 `.gitignore` 忽略）。本仓库对外公开，禁止把 secret key 与真实 Project URL 写入任何被版本控制的文件。
- **免费套餐项目会休眠**：冷启动时首个请求可能较慢或失败，此时由调用方重试；客户端不做自动重试（写操作重试有重复写入风险）。

## MQTT 多节点消息同步（公共 broker）

用于把部署在多台机器上的实例连成一个轻量集群：任一节点发送的消息被其它节点订阅、在界面展示并落库。实现见 `chaos-go/internal/mqttsync`；对外暴露 `/api/mqttSync/*` 路由。

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `MQTT_ENABLED` | 功能总开关（true/false） | `false` |
| `MQTT_BROKER` | broker 地址，如 `tcp://broker.emqx.io:1883` | `tcp://broker.emqx.io:1883` |
| `MQTT_PREFIX` | 集群公共主题前缀（订阅 `prefix + "#"`，发布 `prefix + channel`） | `test/` |
| `MQTT_CLIENT_ID` | MQTT 客户端 ID，缺省 `chaos-<nodeID>` | - |
| `MQTT_USERNAME` | broker 用户名（公共 broker 多为匿名，留空） | - |
| `MQTT_PASSWORD` | broker 密码 | - |
| `MQTT_ENCRYPT` | 是否启用 AES-256-GCM 载荷加密（true/false） | `false` |
| `MQTT_ENCRYPT_KEY` | 32 字节共享密钥（hex 或 base64 均可），**所有节点必须相同**；缺失或无效时 `MQTT_ENCRYPT=true` 仍按明文运行并记错误日志 | - |

**可用性判定**：`MQTT_ENABLED=true` 且 `MQTT_BROKER` 非空，启动时才连接 broker；否则该通道不启用、不发起任何连接，且不影响其余功能启动与运行。

### 行为要点

- **订阅全部**：每个启用节点订阅 `MQTT_PREFIX + "#"`，集群内任意节点发出的消息都会被本节点收到。
- **去重**：消息带全局唯一 `MsgID`；本机发出的消息在发布路径即时落库，订阅回调丢弃 `node_id == 本机` 的回声，并按 `MsgID` 去重，避免重复落库。
- **界面只展示最新一条**：`GET /api/mqttSync/messages` 按 topic（channel）去重，仅返回每个 topic 最新一条；旧消息全部保留在数据库，不在列表重复出现。
- **优雅降级**：broker 不可达 / 订阅失败仅记日志、不 panic、不阻塞启动；离线时发送仍本地落库，仅广播失败。

### 安全警示（重要）

- ⚠️ **公共 broker + 共享前缀 = 任何知道前缀的人都能订阅该命名空间**。建议启用 AES-256-GCM 加密（`MQTT_ENCRYPT=true` + 所有节点相同的 `MQTT_ENCRYPT_KEY`）：启用后载荷为密文，无密钥者无法读取，且 GCM 校验会拒绝任何篡改 / 伪造注入。
- ⚠️ **密钥即信任根**：`MQTT_ENCRYPT_KEY` 泄露等同全集群失效。务必只写在 `.env`（已被 `.gitignore` 忽略），禁止写入任何版本控制文件。生成：`openssl rand -hex 32`。
- ⚠️ **过渡期（部分节点未启用加密）仍可互通**：启用方会接受明文旧消息，但未启用方**无法读取**启用方发出的密文（会被丢弃）。建议集群内一次性全量启用。
- ⚠️ `MQTT_USERNAME` / `MQTT_PASSWORD` 同样只进 `.env`，禁止写入版本控制文件。

## 配置来源

1. 程序启动时默认加载项目根目录的 `.env` 文件
2. 配置项值覆盖内置默认值（`envDefault`）
3. `.env` 文件缺失时，全部使用内置默认值