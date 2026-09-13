# mqtt-cluster-sync Specification

## Purpose
为 `chaos-lib` 提供基于公共 MQTT broker 的多节点消息共享能力：部署在多台机器上的实例通过约定的公共主题前缀互联，任一节点发送的消息被其它节点订阅、在界面展示并落库，使分散的实例构成一个可互相看到广播信息的轻量集群；broker 不可用时应用仍可正常启动与本地运行。

## Requirements

### Requirement: MQTT cluster configuration

系统 SHALL 从配置读取多节点同步所需的 broker 地址、公共主题前缀、节点凭据与功能开关（含加密占位开关）；当 `MQTT_ENABLED` 为假或 broker 地址缺失时，该通道 MUST 被视为不可用，MUST NOT 发起任何 MQTT 连接，且 MUST NOT 影响其余功能的启动与运行。broker 凭据 MUST NOT 出现在任何被版本控制的文件或日志中。

#### Scenario: Channel enabled by complete configuration

- **WHEN** `MQTT_ENABLED=true` 且 `MQTT_BROKER` 非空
- **THEN** 系统将该通道视为可用并在启动时尝试连接

#### Scenario: Channel disabled by missing configuration

- **WHEN** `MQTT_ENABLED` 为假或 `MQTT_BROKER` 为空
- **THEN** 系统将该通道视为不可用、不发起任何连接，且其余功能正常可用

#### Scenario: Common topic prefix is configurable

- **WHEN** 配置中指定了 `MQTT_PREFIX`
- **THEN** 系统以该前缀作为集群公共主题（缺省为 `test/`；`xieqiansong@qq.com/` 为用户个人使用值），订阅与发布均基于该前缀

#### Scenario: Credentials never exposed

- **WHEN** 系统输出日志或向调用方返回错误
- **THEN** 输出内容中不包含 MQTT 用户名 / 密码的任何片段

### Requirement: Subscribing to the entire cluster topic

每个启用的节点 SHALL 订阅 `MQTT_PREFIX + "#"` 下的全部消息，使集群中任意节点发出的消息都能被本节点接收。订阅失败 MUST NOT 导致进程退出或阻塞启动。

#### Scenario: Node subscribes to all cluster messages

- **WHEN** 节点已连接 broker
- **THEN** 系统订阅 `MQTT_PREFIX + "#"` 主题过滤器，并能收到该前缀下的所有消息

#### Scenario: Subscription failure degrades gracefully

- **WHEN** 订阅请求失败或 broker 断连
- **THEN** 系统记录未连接状态并继续运行，MUST NOT 退出进程或阻断启动

### Requirement: Publishing a message

系统 SHALL 支持从界面（或 API）发送一条文本消息：消息 MUST 在发送节点的本地数据库落库，并 SHOULD 被发布到 `MQTT_PREFIX + channel`（默认 `broadcast`）主题。通道启用但 broker 不可达时，系统 MAY 仅完成本地落库并记日志，MUST NOT 因发布失败而丢失本地记录或返回成功假象给调用方造成误导（应明确返回离线状态）。

#### Scenario: Publish stores locally and broadcasts

- **WHEN** 通过 `POST /api/mqttSync/messages` 发送一条消息且 broker 在线
- **THEN** 该消息写入本地数据库，并被发布到集群主题，其它节点能够收到

#### Scenario: Publish while broker offline

- **WHEN** 通道启用但 broker 不可达时发送消息
- **THEN** 消息仍写入本地数据库，系统记录发布失败日志，并在状态中体现离线

#### Scenario: Publish while channel disabled

- **WHEN** `MQTT_ENABLED=false` 时调用发送接口
- **THEN** 系统返回「未启用」业务错误，MUST NOT 发起连接

### Requirement: Persisting received messages to the database

系统 SHALL 将订阅收到的消息（排除本机自身回声）写入本地数据库，并 SHALL 按消息唯一标识（`MsgID`）去重，避免 QoS 重投导致重复落库。本机发出的消息 MUST NOT 经由订阅回声再次落库（发布路径已即时落库）。

#### Scenario: Received message is stored

- **WHEN** 收到一条来自其它节点的、本地尚未存在的消息
- **THEN** 系统将其写入数据库并在界面可见

#### Scenario: Self-echo is dropped

- **WHEN** 收到一条 `node_id` 等于本机节点标识的消息（即自身回声）
- **THEN** 系统丢弃该消息且 MUST NOT 重复落库

#### Scenario: Duplicate delivery is deduplicated

- **WHEN** 收到一条 `MsgID` 已存在于本地数据库的消息
- **THEN** 系统跳过写入，MUST NOT 产生重复记录

### Requirement: Displaying messages on the UI

前端 SHALL 在独立页面展示本机消息列表（含本机发出与来自其它节点的消息），并 SHALL 展示当前连接状态（启用 / 已连接）、本机节点标识、broker 与公共前缀。对于每个 topic（channel），界面 SHALL 仅展示最新的一条消息；更早的同 topic 消息 MUST 仍保留在数据库中，MUST NOT 在列表中重复出现。前端 SHOULD 以合理频率刷新以呈现其它节点的新消息。

#### Scenario: Message list shows latest per topic

- **WHEN** 用户在界面查看消息列表
- **THEN** 界面按 topic 分组，每个 topic 仅展示最新一条消息（时间倒序），包含本机与对端消息，旧消息不在列表重复出现

#### Scenario: Older messages remain in database

- **WHEN** 同一 topic 收到第二条及以后的消息
- **THEN** 先前的消息仍保留在数据库，仅界面列表更新为该 topic 的最新一条

#### Scenario: Connection status is visible

- **WHEN** 用户在界面查看
- **THEN** 界面展示启用状态、连接状态与节点标识，离线时给出明确提示

#### Scenario: Sending from the UI

- **WHEN** 用户在界面输入文本并点击发送
- **THEN** 该消息经 API 发送，并在列表中即时出现本机消息

### Requirement: Connection status query

系统 SHALL 通过 `GET /api/mqttSync/status` 暴露当前通道状态：`enabled`、`connected`、`broker`、`prefix`、`nodeId`。该接口 MUST 在通道禁用或离线时仍可返回，便于前端判断。

#### Scenario: Status when enabled and connected

- **WHEN** 通道启用且 broker 已连接
- **THEN** 状态返回 `enabled=true`、`connected=true` 与当前节点标识

#### Scenario: Status when offline

- **WHEN** 通道启用但 broker 不可达
- **THEN** 状态返回 `enabled=true`、`connected=false`，且不报错

### Requirement: Encryption is deferred (non-goal for this change)

本期内容加密 SHALL NOT 被实现：`payload` MUST 以明文 JSON 传输与存储。`MQTT_ENCRYPT` 仅作为占位开关存在；置 `true` 时本期仍按明文处理，完整加密留待后续变更。系统 MUST 在文档与界面上提示：明文 + 公共 broker 下任何知道前缀的第三方都能读取内容。

#### Scenario: Plaintext payload transported

- **WHEN** 任一消息被发布或落库
- **THEN** 其 `payload` 字段为明文文本，未经过加密

#### Scenario: Encrypt flag is a no-op placeholder

- **WHEN** `MQTT_ENCRYPT=true`
- **THEN** 系统本期仍按明文处理消息，MUST NOT 因该开关为真而中断或报错
