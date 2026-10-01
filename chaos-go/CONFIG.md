# 配置管理说明

## 概述

项目使用基于 YAML 文件的配置系统，默认从运行目录下的 `configs/config.yaml` 加载配置。文件缺失或解析失败时回退到内置默认值；可用环境变量 `CONFIG_FILE` 指定其它路径。

## 配置文件

### 文件位置

- `configs/config.yaml` - 唯一真实配置文件，程序启动时默认加载（缺失则使用内置默认值）。**已被 `.gitignore` 忽略，禁止提交。**
- `configs/config.example.yaml` - 配置模板，入库供参考；复制为 `configs/config.yaml` 后填入实际值。

### 配置项说明

完整字段与默认值见 [`configs/config.example.yaml`](./configs/config.example.yaml)。下面按段落说明。

#### server - 服务器配置

- `server.host` - HTTP 服务地址（默认：`0.0.0.0`）
- `server.port` - HTTP 服务端口（默认：`8080`）

#### db - 数据库配置

- `db.type` - 数据库类型：`postgres` / `sqlite`（默认：`postgres`，切换只需改 type）
- `db.host` / `db.port` / `db.user` / `db.password` / `db.name` / `db.sslmode` - postgres 模式连接参数
- `db.path` - sqlite 模式下的数据库文件路径（相对运行目录，目录自动创建）

#### pprof - 性能分析

- `pprof.enabled` - 是否启用 pprof（默认：`false`）
- `pprof.host` / `pprof.port` - pprof 地址（默认：`localhost:6060`）

#### log - 日志配置

- `log.level` - 日志级别（debug/info/warn/error，默认：`info`）
- `log.file_path` - 日志文件路径（默认：`logs/app.log`）
- `log.to_file` / `log.to_console` - 是否写文件 / 控制台。两者都未显式开启时默认同时开启。

#### feature - 功能开关

- `feature.file_link` - 文件连接功能（默认：`true`）
- `feature.cron_shell` - 定时任务模块的「执行命令/脚本」动作总开关（默认：`false`）；开启后才允许 shell 类型定时任务真正执行命令，否则运行会记录「未启用」

#### deepseek / baidu - 可选 API

- `deepseek.api_key` - DeepSeek API Key（可选，用于余额查询 / AI 评分）
- `baidu.ak` - 百度地图 API Key（可选，用于手机小看板天气）

#### lucky / stun - STUN 相关定时同步

- `lucky.open_token` - Lucky 服务的 openToken，用于请求 stunrulelist 接口
- `lucky.sync_interval_sec` - Lucky STUN 规则同步间隔（秒，默认：`60`）
- `stun.port_forward_sync_interval_sec` - STUN 公网地址同步到端口转发的间隔（秒，默认：`30`）

> 仅当 `mqtt.enabled=true` 且（lucky）已配置 `open_token` 时，对应后台任务才会启动。

#### 定时任务模块

原系统内置的周期任务（任务计划扫描、端口转发自愈、STUN 规则/端口转发同步）已**改为 HTTP 接口**（`POST /api/systemJobs/*`），不再由后台 `scheduler` 自动注册。它们现由独立的**定时任务（cron job）模块**按 cron 表达式触发：首次启动会在库内自动写入 4 条默认任务（可在「定时任务」页面随意改周期/停用）。如需更短于 1 分钟的同步节奏，可在页面把对应任务改为更高频的 cron 表达式（cron 最小粒度为分钟）。

## 新增配置项

如需添加新的配置项：

1. 在 `internal/config/loader.go` 的对应结构体上添加字段，并写好 `yaml:"key"` tag
2. 在 `defaultConfig()` 中给出默认值（如有）
3. 更新 `configs/config.example.yaml`

## 安全注意事项

- ⚠️ **不要**将 `configs/config.yaml` 提交到版本库（已写入 `.gitignore`）
- ✅ 只提交 `configs/config.example.yaml` 作为配置模板
- ⚠️ 真实密码 / Token / 内网地址只写在 `configs/config.yaml`
- ⚠️ 生产环境密码应使用强密码并定期更换

## 使用配置

在代码中获取配置：

```go
import "chaos-go/internal/config"

// 获取完整配置
cfg := config.GetConfig()

// 获取数据库配置
dsn := cfg.Database.GetDSN()

// 获取服务器地址
addr := cfg.Server.GetAddress()

// 检查功能开关
if cfg.Feature.FileLink {
    // 启用文件连接功能
}
```

## Supabase 云端数据通道（Data API / PostgREST）

用于把明确划归云端的数据读写到 Supabase，走纯 HTTPS（不依赖 Postgres 线协议，IPv4 网络即可工作）。
实现见 `chaos-go/internal/supabase`；**不接 HTTP 路由**，因此不会出现在 `/api` 下。

| YAML 键 | 说明 | 默认值 |
|----------|------|--------|
| `supabase.url` | 项目地址，形如 `https://<project-ref>.supabase.co` | - |
| `supabase.secret_key` | `sb_secret_*`，后端专用，绕过 RLS | - |
| `supabase.publishable_key` | `sb_publishable_*`，受 RLS 约束；后端**不使用**，预留给将来的前端只读场景 | - |
| `supabase.schema` | 目标 schema | `public` |
| `supabase.timeout_sec` | 单次请求超时（秒） | `15` |
| `supabase.tables` | 允许访问的表名清单，逗号分隔字符串或 YAML 列表均可（空项自动丢弃） | - |

**可用性判定**：`supabase.url`、`supabase.secret_key`、`supabase.tables` 三者同时非空，该通道才被视为可用；否则 `supabase.Get()` 返回 `nil`，调用方必须判空。

### 注意事项

- **凭据只放 `apikey` 请求头**。Supabase 新式凭据（`sb_publishable_*` / `sb_secret_*`）不是 JWT，放进 `Authorization: Bearer` 会被平台判为 `Invalid JWT`。
- **后端读写必须用 secret key**。publishable key 等价于旧的 `anon`，受行级安全性（RLS）约束：RLS 启用且无策略时读取恒为空集、写入被拒（`42501`）。secret key 具备 `bypassrls`，只要请求不携带用户 access token 就绕过 RLS，无需为每张表编写策略。
- **表名白名单**：不在 `supabase.tables` 中的表名会在发出请求前被拒绝，不会产生任何外网请求。
- **更新与删除必须带过滤条件**：`Update` / `Delete` 未携带过滤条件时直接返回错误，避免整表被改写或清空。
- **非 `public` schema**：除在 `supabase.schema` 指定外，还必须先在该项目的 Dashboard → API Settings 中把该 schema 加入 **Exposed schemas**，否则请求不会生效。
- **安全**：`supabase.secret_key` 只写在 `configs/config.yaml`（已被 `.gitignore` 忽略）。本仓库对外公开，禁止把 secret key 与真实 Project URL 写入任何被版本控制的文件。
- **免费套餐项目会休眠**：冷启动时首个请求可能较慢或失败，此时由调用方重试；客户端不做自动重试（写操作重试有重复写入风险）。

## MQTT 多节点消息同步（公共 broker）

用于把部署在多台机器上的实例连成一个轻量集群：任一节点发送的消息被其它节点订阅、在界面展示并落库。实现见 `chaos-go/internal/mqttsync`；对外暴露 `/api/mqttSync/*` 路由。

| YAML 键 | 说明 | 默认值 |
|----------|------|--------|
| `mqtt.enabled` | 功能总开关（true/false） | `false` |
| `mqtt.broker` | broker 地址，如 `tcp://broker.emqx.io:1883` | `tcp://broker.emqx.io:1883` |
| `mqtt.prefix` | 集群公共主题前缀（订阅 `prefix + "#"`，发布 `prefix + channel`） | `test/` |
| `mqtt.client_id` | MQTT 客户端 ID，缺省 `chaos-<nodeID>` | - |
| `mqtt.username` | broker 用户名（公共 broker 多为匿名，留空） | - |
| `mqtt.password` | broker 密码 | - |
| `mqtt.encrypt` | 是否启用 AES-256-GCM 载荷加密（true/false） | `false` |
| `mqtt.encrypt_key` | 32 字节共享密钥（hex 或 base64 均可），**所有节点必须相同**；缺失或无效时 `mqtt.encrypt=true` 仍按明文运行并记错误日志 | - |

**可用性判定**：`mqtt.enabled=true` 且 `mqtt.broker` 非空，启动时才连接 broker；否则该通道不启用、不发起任何连接，且不影响其余功能启动与运行。

### 行为要点

- **订阅全部**：每个启用节点订阅 `mqtt.prefix + "#"`，集群内任意节点发出的消息都会被本节点收到。
- **去重**：消息带全局唯一 `MsgID`；本机发出的消息在发布路径即时落库，订阅回调丢弃 `node_id == 本机` 的回声，并按 `MsgID` 去重，避免重复落库。
- **界面只展示最新一条**：`GET /api/mqttSync/messages` 按 topic（channel）去重，仅返回每个 topic 最新一条；旧消息全部保留在数据库，不在列表重复出现。
- **优雅降级**：broker 不可达 / 订阅失败仅记日志、不 panic、不阻塞启动；离线时发送仍本地落库，仅广播失败。

### 安全警示（重要）

- ⚠️ **公共 broker + 共享前缀 = 任何知道前缀的人都能订阅该命名空间**。建议启用 AES-256-GCM 加密（`mqtt.encrypt=true` + 所有节点相同的 `mqtt.encrypt_key`）：启用后载荷为密文，无密钥者无法读取，且 GCM 校验会拒绝任何篡改 / 伪造注入。
- ⚠️ **密钥即信任根**：`mqtt.encrypt_key` 泄露等同全集群失效。务必只写在 `configs/config.yaml`（已被 `.gitignore` 忽略），禁止写入任何版本控制文件。生成：`openssl rand -hex 32`。
- ⚠️ **过渡期（部分节点未启用加密）仍可互通**：启用方会接受明文旧消息，但未启用方**无法读取**启用方发出的密文（会被丢弃）。建议集群内一次性全量启用。
- ⚠️ `mqtt.username` / `mqtt.password` 同样只进 `configs/config.yaml`，禁止写入版本控制文件。

## 配置来源

1. 程序启动时默认加载运行目录下的 `configs/config.yaml`（可用 `CONFIG_FILE` 环境变量覆盖路径）
2. 文件中的字段覆盖 `defaultConfig()` 中的内置默认值
3. `configs/config.yaml` 文件缺失时，全部使用内置默认值
