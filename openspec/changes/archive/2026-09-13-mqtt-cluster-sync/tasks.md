## 1. 配置层（chaos-go/config/env.go）

- [ ] 1.1 新增 `MQTTConfig` 结构体（`Enabled` / `Broker` / `Prefix` / `ClientID` / `Username` / `Password` / `Encrypt`）并挂到 `AppConfig.MQTT`；验证：`cd chaos-go && go build ./...` 通过
- [ ] 1.2 `setConfigValue` 增加 7 个解析分支（`MQTT_ENABLED` 用 `parseBool`，其余为字符串）；验证：临时 `.env` 逐项赋值，实测 `config.GetConfig().MQTT` 七个字段正确填充
- [ ] 1.3 `setDefaults` 补齐兜底：`Broker` 缺省 `tcp://broker.emqx.io:1883`、`Prefix` 缺省 `test/`（个人值 `xieqiansong@qq.com/` 由用户写在 `.env`）、`Encrypt` 缺省 `false`；验证：不写这些 key 时得到默认值
- [ ] 1.4 提供可用性判定（`Enabled && Broker!=""` 才视为可用）；验证：四种组合（全空 / 仅 Enabled / 仅 Broker / 齐全）实测判定结果
- [ ] 1.5 `.env.example` 补 MQTT 占位项（仅占位符，无真实凭据）；验证：`git check-ignore -v .env.example` 应无输出，且文件中不含真实 broker 凭据

## 2. 模型与节点标识（chaos-go/internal/mqttsync/message.go）

- [ ] 2.1 定义 `MqttSyncMessage`（`ID`/`MsgID`(uniqueIndex)/`NodeID`/`Channel`/`Payload`/`CreatedAt`/`IsDeleted`），及 `MqttSyncNode`（`ID`/`NodeID`(uniqueIndex)/`CreatedAt`）；验证：`go build ./...` 通过，列名遵循 snake_case、无 `gorm:"column:xxx"`
- [ ] 2.2 实现 `ListLatestPerChannel()`：子查询取 `MAX(id)` per `channel`（`is_deleted=false`），再 `ORDER BY created_at DESC`，返回每个 channel 的最新一条；**取消分页**（旧消息全部保留在数据库）
- [ ] 2.3 实现 Gin handler：`GET /messages`（分页）、`POST /messages`（解析 `{channel?,payload}`，channel 缺省 `broadcast`）、`GET /status`；验证：`go build ./...` 通过

## 3. MQTT 客户端与订阅（chaos-go/internal/mqttsync/mqtt.go）

- [ ] 3.1 封装 `paho.mqtt.golang` 连接：读取 `config.GetConfig().MQTT`，`AutoReconnect=true`；`ClientID` 缺省 `chaos-<NodeID>`；空用户名/密码时不设；验证：`go build ./...` 通过
- [ ] 3.2 `Start()`：仅 `Enabled` 时执行；优先从 `mqtt_sync_node` 加载 `NodeID`，为空则 `uuid.New()` 插入；随后连接并订阅 `Prefix + "#"`；连接/订阅失败仅 `slog.Warn` + 置内存 `connected=false`，MUST NOT panic/阻塞；验证：单元/集成测试覆盖「禁用时空转」「连接失败不阻塞」
- [ ] 3.3 订阅回调：解析 JSON，若 `node_id==本机` 丢弃；否则按 `MsgID` 唯一索引去重写入 `MqttSyncMessage`；验证：测试断言回声丢弃与重复去重
- [ ] 3.4 发布路径：先在 `POST /messages` handler 中以 `NodeID=本机` 落库，再 `Publish(Prefix+channel, json)`；broker 离线时发布失败仅记日志、DB 已落；验证：测试断言「本地落库成功、发布失败不回滚」
- [ ] 3.5 提升 `github.com/eclipse/paho.mqtt.golang` 与 `github.com/google/uuid` 为 direct 依赖并 `go mod tidy`；验证：`go.mod` 中二者不再为 `// indirect`，`go build ./...` 通过

## 4. 启动接线与路由（chaos-go/cmd/server/main.go、routes/routes.go、sql/）

- [ ] 4.1 `main.go` 的 `AutoMigrate()` 追加 `&mqttsync.MqttSyncMessage{}`、`&mqttsync.MqttSyncNode{}`；在 `scheduler.Start()` 之前调用 `mqttsync.Start()`；验证：`go build ./...` 启动后表被创建（`data/chaos.db` 或 postgres 中可见）
- [ ] 4.2 `routes/routes.go` 注册 `mqttSync` 路由组（`GET /messages`、`POST /messages`、`GET /status`）；验证：`go build ./...` + 启动后 `curl` 三个接口可达
- [ ] 4.3 `sql/chaos_postgres_update.sql` 追加 `mqtt_sync_message` / `mqtt_sync_node` 建表语句（日期 + 用途，格式对齐现有条目）；验证：文件末尾新增两条 `CREATE TABLE IF NOT EXISTS`，字段与 GORM 模型一致

## 5. 前端（chaos-ui）

- [ ] 5.1 `utils/api.ts` 增加 `getMqttMessages(page,pageSize)` / `sendMqttMessage(payload,channel?)` / `getMqttStatus()` 三个函数（走 `sendMessage`）
- [ ] 5.2 新增 `views/MqttSync.vue`：连接状态徽标（enabled/connected）、`nodeId`/broker/prefix 只读展示、消息列表（按 topic 分组、每 topic 仅最新一条、时间倒序、无分页、定时轮询 ~3s）、发送输入框 + 按钮；验证：`pnpm build` 通过、组件可挂载
- [ ] 5.3 `router/index.ts` 的 `appRoutes` 追加 `/mqttSync`（`meta.title:'MQTT同步'`、`meta.icon:'Connection'`）；验证：侧边菜单出现「MQTT同步」入口

## 6. 文档

- [ ] 6.1 `chaos-go/CONFIG.md` 补充 MQTT 配置项说明，并写明两条警示：① 明文 + 公共 broker 任何知道前缀的人都能读；② 加密为后续变更；验证：文档默认值与 `setDefaults` 一致
- [ ] 6.2 确认 `chaos_postgres_schema.sql` 未被改动（本期只动 `_update.sql`）；验证：`git status --short` 不含该文件

## 7. 构建与链路验证

- [ ] 7.1 `cd chaos-go && go build ./...` 无编译错误；验证：命令输出为空
- [ ] 7.2 `go vet ./...`（或 `go test -vet=off ./...`，参考历史坏包）与 `gofmt` 检查相关新增文件；验证：新增文件无告警
- [ ] 7.3 `cd chaos-go && go test ./...`（全仓）通过；验证：全部包 `ok`，无 `FAIL`
- [ ] 7.4 后端测试策略（按 AGENTS.md：是否补 `_test.go` 由用户决定，默认建议补充）：用本地内存 broker 或 `paho` 打桩覆盖 3.2–3.4（禁用空转、连接失败不阻塞、回声丢弃、重复去重、离线仍落库）；集成连通性测试仅当显式设置 `MQTT_BROKER` 时真实连接公共 broker，否则 `t.Skip`；验证：`go test ./internal/mqttsync/ -v` 符合预期
- [ ] 7.5 端到端演示（需用户环境）：在 `chaos-go/.env.dev` 置 `MQTT_ENABLED=true` + 选好 `MQTT_BROKER`；启动两个实例（或同机两个进程不同端口）在界面 `/mqttSync` 互发，确认对端收到并落库；验证：实例 A 发送后实例 B 列表出现该消息

## 8. 前端手动验证清单（用户人工验收）

- [ ] 8.1 未启用（`MQTT_ENABLED=false`）时访问 `/mqttSync`，界面提示「未启用」，发送报错且不发起连接
- [ ] 8.2 启用且 broker 在线时：状态徽标显示「已连接」、展示 `nodeId` 与 `prefix`
- [ ] 8.3 在 A 实例输入文本发送，A 列表即时出现本机消息；约 3s 内 B 实例列表出现来自 A 的消息
- [ ] 8.4 刷新页面 / 重启某实例后，历史消息仍在（已落库）
- [ ] 8.5 broker 不可达时：状态显示「离线」，A 仍能本地发送并落库（离线提示），不崩溃
- [ ] 8.6 同一 topic 连续发送多条：界面始终只显示该 topic 最新一条，旧消息仍在数据库（可经 DB/查询确认）

## 9. 决策与实施记录（AI 填写）

- [ ] 9.1 记录第 7.4 节测试决策：是否补充 `_test.go`、用户答复、拒绝时的替代验证方式
- [ ] 9.2 记录第 7.5 节端到端演示结果（broker 域名、两实例端口、互发是否成功）
- [ ] 9.3 记录环境阻塞项（公共 broker 限流 / 网络不可达等）
- [ ] 9.4 记录遗留项：`MQTT_ENCRYPT` 未实现；多进程同机共享 `mqtt_sync_node` 场景未处理

## 10. 本次改动文件清单

- `chaos-go/config/env.go`（改）：`MQTTConfig`、`AppConfig.MQTT`、7 个解析分支、默认值
- `chaos-go/internal/mqttsync/message.go`（新增）：模型 + handler + 列表分页
- `chaos-go/internal/mqttsync/mqtt.go`（新增）：client + 订阅回调 + `Start()`
- `chaos-go/internal/mqttsync/mqtt_test.go`（新增，若确认）：单元/集成测试
- `chaos-go/cmd/server/main.go`（改）：`AutoMigrate` 登记 + `mqttsync.Start()`
- `chaos-go/routes/routes.go`（改）：`/api/mqttSync` 路由组
- `chaos-go/sql/chaos_postgres_update.sql`（改）：两张表建表语句
- `chaos-go/.env.example`（改）：MQTT 占位项
- `chaos-go/CONFIG.md`（改）：MQTT 配置说明 + 安全警示
- `chaos-ui/src/utils/api.ts`（改）：3 个请求函数
- `chaos-ui/src/views/MqttSync.vue`（新增）：页面
- `chaos-ui/src/router/index.ts`（改）：`/mqttSync` 路由
- 未改动（已核对）：`chaos-go/sql/chaos_postgres_schema.sql`、其它 `internal/*` 模块
