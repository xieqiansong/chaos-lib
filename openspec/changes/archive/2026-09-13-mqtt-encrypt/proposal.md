# Change: MQTT 消息内容加密（AES-256-GCM）

## Why

原 `mqtt-cluster-sync` 变更将加密列为 Non-Goal：`payload` 以明文在公共 MQTT broker 上传输与存储。由于集群使用共享前缀（如 `xieqiansong@qq.com/#`），任何知道前缀的第三方都能**读取内容并注入假消息**。用户决定现在补全加密，使跨机共享信息在公共 broker 上具备机密性与抗伪造能力。

## What Changes

- 引入 **AES-256-GCM** 对称认证加密（Go 标准库，无新依赖）：发布前加密载荷，订阅回调解密；解密失败（被篡改 / 无密钥注入）的消息直接丢弃。
- 新增配置项 `MQTT_ENCRYPT_KEY`（32 字节，hex 或 base64，来自 `.env`，绝不入库 / 入日志）；`MQTT_ENCRYPT=true` 时启用。
- 线上采用统一信封：`{"v":1,"enc":"aes-256-gcm","data":"<base64(nonce‖密文+tag)>"}`；无 `enc` 字段的旧消息按明文兼容处理（便于集群滚动升级）。
- 移除原 spec 中「Encryption is deferred」的 Non-Goal 条目；新增「Payload encryption」与「Encryption key configuration」两条需求。
- `GET /api/mqttSync/status` 增加 `encrypt` 布尔字段，前端可展示加密徽标。
- **不改动数据库结构**：本地 DB 仍存明文（本地可信），仅传输层加密。

## Impact

- `chaos-go/internal/mqttsync/mqtt.go`：新增加密 / 解密封装与信封处理，接入发布与订阅路径。
- `chaos-go/config/env.go` + `chaos-go/.env.example` + `chaos-go/CONFIG.md`：新增 `MQTT_ENCRYPT_KEY`。
- `chaos-go/internal/mqttsync/mqtt.go`：`GET /status` 返回 `encrypt`。
- `chaos-ui/src/views/MqttSync.vue`：可选展示加密状态徽标。
- `openspec/specs/mqtt-cluster-sync/spec.md`：删除延后条目、新增加密需求（归档时合并）。
- 无破坏性变更，无数据库迁移。

## Migration

- 所有节点在 `.env` 写入**相同**的 `MQTT_ENCRYPT_KEY` 并置 `MQTT_ENCRYPT=true` 后重启即可；过渡期内未启用节点发出的明文消息仍可被启用节点接收（反之亦然），启用后全部流量即加密。
- 回滚：置 `MQTT_ENCRYPT=false` 即退回明文，无需其它操作。
