## Context

- 复用已归档 `mqtt-cluster-sync` 的模块 `chaos-go/internal/mqttsync`：发布路径先落库再 `Publish`，订阅回调按 `node_id` 去回声、`MsgID` 去重后落库（见原 design D3/D5/D6）。
- 当前线上 payload 为明文 JSON：`{"id":...,"node_id":...,"channel":...,"payload":...,"ts":...}`，发布主题为 `Prefix + channel`。加密**只作用于该 JSON 的传输形态**，不改变业务字段与落库内容。
- Go 标准库 `crypto/aes`、`crypto/cipher`（GCM）、`crypto/rand`、`encoding/base64`、`encoding/hex` 已齐备，无新第三方依赖。

## Goals / Non-Goals

**Goals:**
- `MQTT_ENCRYPT=true` 且密钥有效时，broker 上传输的 payload 为 AES-256-GCM 密文，第三方无法读取或伪造
- 解密失败（篡改 / 无密钥注入）的消息被静默丢弃并记日志
- 密钥仅来自 `.env`，绝不出现在任何版本控制文件或日志
- 过渡期兼容明文旧消息（信封检测）

**Non-Goals（相对本期）:**
- 不做非对称 / 每节点独立密钥（小集群共享对称密钥足够）
- 不改数据库结构（本地存明文）
- 不对 topic 中的 `channel` 做隐藏（其仅作为分类名出现在主题层级，属非敏感元数据；如需完全隐藏可后续改为统一发往 `prefix+broadcast` 并把 channel 收进加密体，本期不改）

## Decisions

### D10: 算法与信封格式
- 算法 **AES-256-GCM**：32 字节密钥（AES-256），12 字节随机 nonce，输出 16 字节 tag（GCM 默认）。
- 封装流程（发布）：
  1. 构造内层业务 JSON（与原结构一致）
  2. `nonce = crypto/rand` 取 12 字节
  3. `ct = AESGCM.Seal(nil, nonce, innerJSON, nil)`
  4. `data = base64(nonce ‖ ct)`（`ct` 已含 tag）
  5. 外层信封：`{"v":1,"enc":"aes-256-gcm","data":data}`，marshal 后发布
- 解封流程（订阅）：
  1. `json.Unmarshal(raw)`；若得到对象含 `enc=="aes-256-gcm"` → 走解密路径
  2. `base64 decode data` → 拆出前 12 字节 nonce 与余下 `ct`
  3. `AESGCM.Open(nil, nonce, ct, nil)`；**失败 → 返回错误（调用方丢弃该消息）**
  4. 得到内层业务 JSON，继续走去回声 / 去重 / 落库
  5. 若 `enc` 字段缺失 → 视为明文旧消息，直接当作内层 JSON 处理（兼容）

### D11: 密钥管理
- `config.MQTTConfig` 新增 `EncryptKey string`（对应 `MQTT_ENCRYPT_KEY`）。
- 取值支持两种编码：优先按 hex 解析（须 64 字符 → 32 字节）；失败再按 base64 解析（须解出 32 字节）；否则视为无效。
- 生成方式（文档写入 `.env.example` 注释）：`openssl rand -hex 32`。
- 所有节点必须配置**同一把**密钥；密钥与 username/password 一样 MUST NOT 入库、MUST NOT 进日志。

### D12: 启用判定与降级
- `mqttsync.Start()` 内计算 `effectiveEncrypt = cfg.Encrypt && validKey()`：
  - `cfg.Encrypt=false` → 不加密（原明文行为）
  - `cfg.Encrypt=true` 且密钥无效 / 缺失 → `slog.Error("MQTT_ENCRYPT=true 但 MQTT_ENCRYPT_KEY 无效，本会话按明文运行")` 并令 `effectiveEncrypt=false`，**仍照常运行**（可用性优先，但明确告警）
  - `cfg.Encrypt=true` 且密钥有效 → 启用加密
- 解密失败的订阅消息：`slog.Warn` 后丢弃，MUST NOT 落库、MUST NOT 中断连接。
- `GET /status` 返回 `encrypt: effectiveEncrypt`，便于前端判断是否真正加密。

### D13: 测试（无需 broker）
- 新增 `internal/mqttsync/mqtt_test.go`（纯函数，不连 broker）：
  - 固定密钥：加密 → 解密 往返得到原文
  - 篡改密文任一字节 → 解密失败
  - 用错误密钥解密 → 失败
  - 无 `enc` 字段的明文消息 → 正常按明文解析（兼容路径）
- 不依赖公共 broker，稳定可重复。

## Risks / Trade-offs

- **密钥共享 surfaced**：多机共享同一对称密钥，任一台 `.env` 泄露即全集群失效——这是对称方案的固有代价，文档须强调密钥保护。
- **过渡期明文窗口**：滚动升级期间启用 / 未启用节点混跑，未启用方发出明文可被读；文档建议「一次性全量启用」。
- **channel 仍见于主题**：`Prefix + channel` 主题为明文，channel 名对订阅者可见（非敏感）；若需彻底隐藏可后续调整，本期不改。

## Open Questions

- 无（原 Open Question「加密方案与密钥来源」在此变更中确定：AES-256-GCM + `.env` 共享密钥）。
