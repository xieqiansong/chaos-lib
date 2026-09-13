## Context

- 后端 `chaos-go`（module `chaos-go`，Go 1.26）：Gin + GORM，业务逻辑放 `internal/<模块>/`（模型 + handler 同包），配置走 `config` 单例（`config.GetConfig()`），DB 走 `config.GetDB()`，新模型在 `cmd/server/main.go` 的 `AutoMigrate()` 登记，`sql/chaos_postgres_update.sql` 记增量
- 前端 `chaos-ui`：Vue 3 + TS + Vite + Element Plus + hash 路由；请求统一走 `utils/api.ts` 的 `sendMessage`；新页面在 `router/index.ts` 的 `appRoutes` 追加一条，`meta.title`/`meta.icon` 即菜单来源
- 既有 `internal/supabase` 是「云端数据通道」的参照范式：配置驱动单例、可用性判定、凭据不泄露、不阻塞启动。本期 MQ 通道复用同一套「配置可用才启用、不可用则静默」的退化思路，但**新增 HTTP 路由与 GORM 模型**（与 supabase 不同，supabase 当时不暴露给前端）
- 依赖现状：`github.com/eclipse/paho.mqtt.golang v1.5.1` 与 `github.com/google/uuid v1.3.0` 已作为 indirect 出现在 `go.mod`，本期提升为 direct 即可，无需引入新第三方源
- 分页：复用 `chaos-go/internal/pagination`（既有统一分页），`GET /messages` 返回 `{list, total, page, page_size}`

## Goals / Non-Goals

**Goals:**
- 多节点通过公共 MQTT broker 共享消息：任一节点发送，其余节点都能收到并落库
- 每个节点订阅集群前缀下的**全部**主题（`prefix + "#"`），并在界面展示
- 收到的消息（含自身发出的）同步写入本地 SQLite/PostgreSQL
- 界面提供发送入口（测试用）与连接状态展示
- broker 不可用时优雅降级：应用照常启动、本地功能不受影响，仅 MQ 通道离线

**Non-Goals:**
- **不实现内容加密**：`payload` 明文传输与存储；`MQTT_ENCRYPT` 仅占位，置 `true` 时本期仍按明文处理
- 不做消息可靠投递保证（QoS 仅用 broker 默认 0/1，不做本地离线队列重发）
- 不做多 broker 高可用 / 故障转移
- 不做 topic 级别的权限隔离（公共 broker 下前缀即共享命名空间）

## Decisions

### D1: 模块落在 `internal/mqttsync`，模型 + handler 同包
- `mqtt.go`：封装 `paho.mqtt.golang` 连接 / 订阅 / 发布 + 节点标识加载
- `message.go`：`MqttSyncMessage` / `MqttSyncNode` 模型 + Gin handler
- 不引入依赖注入，沿用 `config.GetDB()` / `config.GetConfig()` 单例

### D2: 配置项（全部进 `config.MQTTConfig`）

| key | 字段 | 默认 | 说明 |
|---|---|---|---|
| `MQTT_ENABLED` | `Enabled` | `false` | 功能总开关 |
| `MQTT_BROKER` | `Broker` | `tcp://broker.emqx.io:1883` | broker 地址（公共免费） |
| `MQTT_PREFIX` | `Prefix` | `test/` | 集群公共主题前缀（用户个人值 `xieqiansong@qq.com/` 写在 `.env`） |
| `MQTT_CLIENT_ID` | `ClientID` | 空 | 为空时自动用 `chaos-<nodeID>` |
| `MQTT_USERNAME` | `Username` | 空 | 公共 broker 多为匿名，留空 |
| `MQTT_PASSWORD` | `Password` | 空 | 同上 |
| `MQTT_ENCRYPT` | `Encrypt` | `false` | 占位开关，本期不实现加密 |

前缀以 `/` 结尾；订阅 filter 为 `Prefix + "#"`，发布主题为 `Prefix + "broadcast"`（`channel` 可覆盖，缺省 `broadcast`）。

### D3: 主题与消息结构
- 订阅 filter：`xieqiansong@qq.com/#` → 匹配 `xieqiansong@qq.com/<任意层级>`
- 发布主题：`xieqiansong@qq.com/broadcast`（或 `Prefix + channel`）
- 线上 payload 为明文 JSON：`{"id":"<uuid>","node_id":"<本节点>","channel":"broadcast","payload":"<文本>","ts":"<RFC3339>"}`
- `MQTT_ENCRYPT=true` 时本期**仍按明文**发送/存储（分支未实现），仅留此字段作为后续变更的接入点

### D4: 节点标识 `MqttSyncNode`
- 启动（`mqttsync.Start()`）时加载节点标识：若 `mqtt_sync_node` 表为空则插入一条 `NodeID = uuid.New()` 的新行，否则读取已存在的 `NodeID`
- `NodeID` 全程稳定（用于去重与界面「本机」标识）；`MQTT_CLIENT_ID` 缺省为 `chaos-<NodeID>`
- 模型字段：`ID uint` PK、`NodeID string`（`uniqueIndex`）、`CreatedAt`
- **注意**：不用 `gorm:"column:xxx"`（遵循 AGENTS.md），列名自动 snake_case

### D5: 去重策略（避免自己收到自己发出的回声）
- **发布路径**：先以 `NodeID=本机` 落库（本地即时可见），再 `Publish` 到 broker
- **订阅回调**：收到消息后解析 `node_id`；若 `node_id == 本机 NodeID` 则直接丢弃（回声）；否则按 `id`（MsgID）做唯一写入——已存在则跳过，防止 QoS 重投重复落库
- 因此本地 DB 最终包含「本机发出的 + 其它节点发来的」全部消息，无重复

### D6: 消息落库模型 `MqttSyncMessage`
字段：`ID uint` PK、`MsgID string`（`uniqueIndex`，即线上 JSON 的 `id`）、`NodeID string`（发送方）、`Channel string`、`Payload string`、`CreatedAt time.Time`、`IsDeleted bool`（软删除，遵循仓库约定，查询加 `WHERE is_deleted = ?`）
- 列表 `GET /messages` 按 `channel` 去重，**仅返回每个 channel 的最新一条**：用子查询取 `MAX(id)` per channel（`id` 自增，最大即最新），再 `ORDER BY created_at DESC`；**取消分页**——旧消息全部保留在数据库，不在界面列表重复展示

### D7: 优雅降级
- `mqttsync.Start()` 仅在 `MQTT_ENABLED=true` 时真正连接；连接失败 / 订阅失败仅 `slog.Warn` + 置内存状态 `connected=false`，**绝不 panic、绝不阻塞 `main`**
- `GET /status` 返回 `{enabled, connected, broker, prefix, nodeId}`；`connected=false` 时 `POST /messages` 仍会把消息写入本地 DB（`Enabled` 为真时尝试发布，失败仅记日志、不回滚 DB），保证「本地可用、广播尽力而为」
- `Enabled=false` 时 `POST /messages` 返回 `disabled` 业务错误，前端提示未启用

### D8: 前端 `MqttSync.vue`
- 页头展示连接状态徽标（enabled / connected）、当前 `nodeId`、broker 与 prefix（脱敏只读）
- 消息列表：调用 `GET /api/mqttSync/messages`，按 topic 分组展示每个 topic 最新一条（时间倒序），定期轮询（如 3s）以呈现其它节点新消息；不再分页
- 发送区：一个文本输入框 + 「发送」按钮，调用 `POST /api/mqttSync/messages`；成功后在列表中即时看到本机消息
- 路由 `/mqttSync`，`meta.title:'MQTT同步'`，`meta.icon:'Connection'`（与端口转发同款图标，或 `Share`）

### D9: 启动接线
- `cmd/server/main.go`：在 `AutoMigrate()` 的模型列表中追加 `&mqttsync.MqttSyncMessage{}`、`&mqttsync.MqttSyncNode{}`；在 `scheduler.Start()` 之前调用 `mqttsync.Start()`（内部判 `Enabled`，未启用则空转）
- 不改动既有 `AutoMigrate` 之外的启动顺序，不影响其它模块

## Risks / Trade-offs

- **明文 + 公共 broker 的隐私风险（最高）**：`xieqiansong@qq.com/#` 是任何人都能订阅的共享命名空间，本期明文 payload 可被任意第三方读取/注入。这是用户已知并推迟处理的项；design 标注、spec 单列一条 Non-Goal，但**强烈建议正式使用前补加密变更**。公共 broker 速率/连接数限制也可能影响多节点稳定性
- **公共 broker 不稳定**：免费 broker 可能限流、断连。`paho` 自带 `AutoReconnect`，本期开启；`connected` 状态展示帮助用户判断
- **消息顺序 / 重复**：QoS 0/1 下顺序不严格保证；靠 `MsgID` 唯一索引去重已覆盖重复，顺序以 `created_at` 近似
- **回声与多实例同一 DB**：同一台机器跑多个进程会共享 `mqtt_sync_node` 表，可能拿到不同 `NodeID`。本期假设每机一个进程；多进程场景后续再议
- **轮询而非 SSE/WebSocket**：前端用定时轮询简化实现，足够演示；高实时性需求后续可升级

## Migration Plan

- 无破坏性变更：仅新增两张表（自动迁移），新增 `.env` 配置项（缺省关闭）
- 回滚：置 `MQTT_ENABLED=false` 即可停用 MQ 通道；如需彻底移除，删除两张表与路由即可，不影响其它模块
- 部署：后端重新 `go build`，前端 `pnpm build` 后由 `scripts/chaos.deploy.ps1` 内嵌

## Open Questions

- 加密方案（AES-GCM + 共享口令？非对称？）留待后续变更，需约定密钥来源（`.env` vs 其它）
- 选用哪个公共 broker、是否需要用户名/密码，待用户实测后定；`.env.example` 默认给 `broker.emqx.io`
