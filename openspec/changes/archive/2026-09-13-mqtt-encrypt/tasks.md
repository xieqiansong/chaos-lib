# Tasks

## 1. 配置项与文档
- [x] `config/env.go`：`MQTTConfig` 新增 `EncryptKey string`，绑定 `MQTT_ENCRYPT_KEY`
- [x] `chaos-go/.env.example`：补充 `MQTT_ENCRYPT` / `MQTT_ENCRYPT_KEY` 及 `openssl rand -hex 32` 生成说明
- [x] `chaos-go/CONFIG.md`：补充加密配置段落

## 2. 加密实现（mqtt.go）
- [x] 新增 `loadKey()`：hex→base64 解析为 32 字节，返回是否有效
- [x] 新增 `seal` / `open`（AES-256-GCM + 信封）
- [x] `Start()` 计算 `effectiveEncrypt`，密钥无效时 `slog.Error` 并降级为明文
- [x] 发布路径：业务 JSON → `seal()` → 信封 → `Publish`
- [x] 订阅回调：先 `open()`，失败则 `slog.Warn` 丢弃；成功再走去回声 / 去重 / 落库

## 3. 状态与前端
- [x] `GET /status` 响应增加 `encrypt`（= `effectiveEncrypt`）
- [x] `chaos-ui/src/views/MqttSync.vue`：状态区展示加密徽标与未加密告警

## 4. 测试与校验
- [x] `internal/mqttsync/mqtt_test.go`：往返、篡改失败、错密钥失败、明文兼容、状态读取
- [x] `go build ./...` 与 `npx @fission-ai/openspec validate --changes` 通过
