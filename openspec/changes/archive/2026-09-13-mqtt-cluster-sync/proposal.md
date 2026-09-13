## Why

`chaos-lib` 目前所有数据都只落在单机的 SQLite / 自建 PostgreSQL，多台机器各自为政。用户要在多台机器上部署同一套应用，并让它们之间共享信息——例如一条在某个节点发出的消息，其它节点都能看到并落库。

实现思路是用一台**免费的公共 MQTT broker**（如 `broker.emqx.io`、`test.mosquitto.org`）作消息总线：整个集群约定一个公共主题前缀（默认 `xieqiansong@qq.com/`，由 `.env` 定义），每个节点订阅该前缀下的全部消息、在界面上展示，并同步写入本地数据库；界面上提供发送能力（测试用）。

内容加密用户明确「之后再考虑」，本次**不实现加密**，但主题与消息结构预留接入点。

## What Changes

- 新增 `chaos-go/internal/mqttsync` 包（模型 + handler 同包，符合 `AGENTS.md` 约定）：封装 MQTT 连接 / 订阅 / 发布，并把收到的消息落库
- 新增 `config.MQTTConfig`（`MQTT_ENABLED` / `MQTT_BROKER` / `MQTT_PREFIX` / `MQTT_CLIENT_ID` / `MQTT_USERNAME` / `MQTT_PASSWORD` / `MQTT_ENCRYPT`），沿用既有 `-env=dev|prod` + `.env.{dev,prod}` 加载方式；前缀缺省 `test/`（`xieqiansong@qq.com/` 为用户个人使用值，写在 `.env` 中）
- 新增 GORM 模型 `MqttSyncMessage`（与 `MqttSyncNode` 节点标识表），在 `cmd/server/main.go` 的 `AutoMigrate()` 登记，并写入 `chaos-go/sql/chaos_postgres_update.sql` 增量日志
- 新增 HTTP 路由组 `/api/mqttSync`：`GET /messages`（按 topic 去重，仅返回每个 topic 最新一条，无分页）、`POST /messages`（发送并广播）、`GET /status`（连接状态与节点标识）
- 启动逻辑：若 `MQTT_ENABLED=true`，在 `main.go` 中调用 `mqttsync.Start()` 连接 broker 并订阅 `prefix + "#"`；连接失败或 broker 不可达时**不阻塞启动**，仅记录状态为未连接
- 前端新增 `chaos-ui/src/views/MqttSync.vue`（路由 `/mqttSync`）：展示连接状态、每个 topic 仅展示最新一条的消息列表、发送输入框；`api.ts` 增加对应请求函数，`router/index.ts` 追加路由
- **不实现加密**：`payload` 本次以明文 JSON 传输与存储；`MQTT_ENCRYPT` 仅作为占位开关预留，置 `true` 时本期仍按明文处理（未实现分支）
- 联通性验证走 Go 测试：用 `paho.mqtt.golang` 的本地 `mqtt.Server` / 内存 broker 或打桩做单元验证；考虑到公共 broker 不稳定，集成测试仅在显式设置 `MQTT_BROKER` 时真实连接，否则 `t.Skip`
- `.env.example` 新增 MQTT 占位项；真实 broker 凭据只进 `.env.dev` / `.env.prod`（已被 `.gitignore` 忽略）

## Capabilities

### New Capabilities

- `mqtt-cluster-sync`: 基于公共 MQTT broker 的多节点消息共享能力，覆盖配置、订阅全部主题、发布消息、消息落库、界面展示与发送、连接状态查询，且 broker 不可用时可优雅降级

### Modified Capabilities

<!-- 无：不改变任何既有能力的功能规格 -->

## Impact

- 新增：`chaos-go/internal/mqttsync/mqtt.go`（client + 订阅回调）、`chaos-go/internal/mqttsync/message.go`（模型 + handler）、`chaos-go/internal/mqttsync/mqtt_test.go`
- 新增：`chaos-ui/src/views/MqttSync.vue`、`chaos-ui/src/api/mqttSync.ts`（或并入 `utils/api.ts`）
- 修改：`chaos-go/config/env.go`（新增 `MQTTConfig` 结构体、`AppConfig` 字段、`setConfigValue` 解析分支、`setDefaults` 默认值）
- 修改：`chaos-go/cmd/server/main.go`（`AutoMigrate` 登记两个模型 + 启动 `mqttsync.Start()`）
- 修改：`chaos-go/routes/routes.go`（注册 `/api/mqttSync` 路由组）
- 修改：`chaos-go/sql/chaos_postgres_update.sql`（追加 `mqtt_sync_message` / `mqtt_sync_node` 建表语句）
- 修改：`chaos-ui/src/router/index.ts`（追加 `/mqttSync` 路由）
- 修改：`chaos-go/.env.example`（新增 MQTT 占位项，无真实 broker 凭据）
- 修改：`chaos-go/CONFIG.md`（补充 MQTT 配置项说明）
- 依赖：`github.com/eclipse/paho.mqtt.golang` 由 indirect 提升为 direct（已在 go.mod 中、`v1.5.1`）；`github.com/google/uuid` 提升为 direct（已在 go.mod 中、`v1.3.0`）；`go mod tidy` 后生效
- 安全约束：本仓库「公开 = 是」，**明文 payload + 公共 broker 意味着任何知道前缀的人都能订阅并读取内容**；这是已知风险，加密为后续变更，本期不处理，仅在 design.md 标注
