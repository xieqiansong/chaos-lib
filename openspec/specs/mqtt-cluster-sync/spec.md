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

- **WHEN** 通过 `POST /api/mqttSync`（新建消息，经 AfterCreate 广播）发送一条消息且 broker 在线
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

前端 SHALL 在独立页面以分页表格展示本机消息列表（含本机发出与来自其它节点的消息），并 SHALL 展示当前连接状态（启用 / 已连接）、本机节点标识、broker 与公共前缀。列表 MUST 由通用 DataTable 驱动（资源接口自包含于 `internal/mqttsync`，遵循标准数据基线），支持按 topic（channel）/ 节点（node_id）/ 内容（payload）搜索与按时间排序；所有消息 MUST 在数据库中保留，删除为软删除。前端 SHOULD 以合理频率刷新状态以呈现集群变化。

#### Scenario: Message list is paginated and searchable

- **WHEN** 用户在界面查看消息列表
- **THEN** 界面以分页表格展示全部消息，可按主题 / 节点 / 内容搜索，并按时间排序，包含本机与对端消息

#### Scenario: Older messages remain in database

- **WHEN** 同一 topic 收到多条消息
- **THEN** 全部消息均保留在数据库，可在列表中分页查看，删除仅标记软删

#### Scenario: Connection status is visible

- **WHEN** 用户在界面查看
- **THEN** 界面展示启用状态、连接状态与节点标识，离线时给出明确提示

#### Scenario: Sending from the UI

- **WHEN** 用户在平铺广播表单填写主题与内容后点击「广播」
- **THEN** 该消息经 API 落库并经 AfterCreate 广播到集群，列表中即时出现本机消息；表单右侧的「删除该主题消息」按该主题批量软删

### Requirement: Connection status query

系统 SHALL 通过 `GET /api/mqttSync/status` 暴露当前通道状态：`enabled`、`connected`、`broker`、`prefix`、`nodeId`。该接口 MUST 在通道禁用或离线时仍可返回，便于前端判断。

#### Scenario: Status when enabled and connected

- **WHEN** 通道启用且 broker 已连接
- **THEN** 状态返回 `enabled=true`、`connected=true` 与当前节点标识

#### Scenario: Status when offline

- **WHEN** 通道启用但 broker 不可达
- **THEN** 状态返回 `enabled=true`、`connected=false`，且不报错

### Requirement: Payload encryption with AES-256-GCM

系统 SHALL 在 `MQTT_ENCRYPT=true` 且配置了有效 `MQTT_ENCRYPT_KEY` 时，对所有发布到 broker 的消息载荷采用 **AES-256-GCM** 对称认证加密；每条消息使用随机 12 字节 nonce，密文与 16 字节 GCM tag 经 base64 封装于外层信封 `{"v":1,"enc":"aes-256-gcm","data":"<base64(nonce‖密文+tag)>"}` 后发布。订阅回调 MUST 在落库前完成解密。该加密仅作用于传输层，MUST NOT 改变本地数据库存储内容（本地仍存明文业务 JSON）。

#### Scenario: Encrypt on publish

- **WHEN** `MQTT_ENCRYPT=true` 且 `MQTT_ENCRYPT_KEY` 有效，本地发送一条消息
- **THEN** 发布到 broker 的内容为加密信封，第三方无法直接读取 `payload`

#### Scenario: Decrypt on receive

- **WHEN** 订阅回调收到一条有效加密信封
- **THEN** 系统用同一密钥成功解密还原业务 JSON，并继续去回声 / 去重 / 落库

#### Scenario: Tampered or forged message is rejected

- **WHEN** 收到的消息密文被篡改，或来自无密钥的第三方伪造
- **THEN** GCM 校验失败，系统 MUST 丢弃该消息并记日志，MUST NOT 落库或中断连接

#### Scenario: Key sourced from environment only

- **WHEN** 系统加载配置或输出日志
- **THEN** `MQTT_ENCRYPT_KEY` 仅来自 `.env`，MUST NOT 写入任何版本控制文件或日志

#### Scenario: Plaintext legacy message is still accepted

- **WHEN** 订阅回调收到一条不含 `enc` 字段的明文旧消息
- **THEN** 系统按明文兼容处理并正常落库，保证集群滚动升级期间互通

### Requirement: Encryption key configuration

系统 SHALL 从 `MQTT_ENCRYPT_KEY` 读取 32 字节共享密钥（支持 hex 或 base64 编码），且仅当 `MQTT_ENCRYPT=true` 时生效；所有集群节点 MUST 配置同一把密钥。当 `MQTT_ENCRYPT=true` 但密钥缺失或格式无效时，系统 MUST 记录错误日志并以明文降级运行（保持可用性），MUST NOT 因此中断启动或连接。

#### Scenario: Valid key enables encryption

- **WHEN** `MQTT_ENCRYPT=true` 且 `MQTT_ENCRYPT_KEY` 为合法 32 字节密钥
- **THEN** 系统启用加密，本会话所有对外发布均为密文

#### Scenario: Missing or invalid key degrades to plaintext with warning

- **WHEN** `MQTT_ENCRYPT=true` 但 `MQTT_ENCRYPT_KEY` 为空或无法解析为 32 字节
- **THEN** 系统记错误日志、以明文运行，且其余功能不受影响

#### Scenario: Status reports encryption state

- **WHEN** 调用 `GET /api/mqttSync/status`
- **THEN** 响应包含 `encrypt` 字段，反映本会话是否真正启用加密
