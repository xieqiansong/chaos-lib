# mqtt-cluster-sync Specification (delta)

## REMOVED Requirements

### Requirement: Encryption is deferred (non-goal for this change)

本期内容加密 SHALL NOT 被实现：`payload` MUST 以明文 JSON 传输与存储。`MQTT_ENCRYPT` 仅作为占位开关存在；置 `true` 时本期仍按明文处理，完整加密留待后续变更。系统 MUST 在文档与界面上提示：明文 + 公共 broker 下任何知道前缀的第三方都能读取内容。

#### Scenario: Plaintext payload transported

- **WHEN** 任一消息被发布或落库
- **THEN** 其 `payload` 字段为明文文本，未经过加密

#### Scenario: Encrypt flag is a no-op placeholder

- **WHEN** `MQTT_ENCRYPT=true`
- **THEN** 系统本期仍按明文处理消息，MUST NOT 因该开关为真而中断或报错

## ADDED Requirements

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
