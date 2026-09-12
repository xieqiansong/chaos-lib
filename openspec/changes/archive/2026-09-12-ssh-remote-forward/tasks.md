## 1. 模型与迁移

- [x] 1.1 `chaos-go/internal/portfwd/portfwd.go`：`PortForwarding` 新增 `Direction`、`BindAddress` 字段；验证方式：`go build ./...` 通过且未添加 `gorm:"column:..."` 标签
- [x] 1.2 规范化辅助：方向空值 → `local`；监听地址空值按方向取缺省（`local`=`0.0.0.0`，`remote`=`127.0.0.1`）；验证方式：实测省略 `Direction` 创建出 `local`/`0.0.0.0`，省略 `BindAddress` 的 `remote` 规则得到 `127.0.0.1`
- [x] 1.3 `chaos-go/sql/chaos_postgres_update.sql` 追加本条变更的增量 DDL（`ALTER TABLE port_forwarding ADD COLUMN IF NOT EXISTS direction text` / `bind_address text` + 存量回填 `direction='local'`，注明日期、用途与「空值视为 local」语义）；验证方式：人工比对模型字段与 DDL 列一一对应（**未修改 `chaos_postgres_schema.sql`**）
- [x] 1.4 确认 `AutoMigrate` 无需改动（`PortForwarding` 已在登记列表）；验证方式：实测新库中规则的 `Direction`/`BindAddress` 可正常读写，说明两列已建成

## 2. 双向隧道引擎（internal/portfwd/forwarder.go）

- [x] 2.1 抽出「监听策略」：按 `Direction` 选择 `net.Listen`（local）或 `ssh.Client.Listen`（remote）；验证方式：代码实现完成，两个方向的实际监听需真实 SSH 凭据验证（见 7.2 / 7.3）
- [x] 2.2 本地方向保持既有语义：listener 一次创建、跨重连存活；验证方式：实现保持一致（`localAcceptLoop` 持有稳定 listener），跨重连行为见 7.10
- [x] 2.3 远程方向引入 supervisor：外层串行 `Listen`，内层 accept 循环按 listener 代际运行；同一时刻只允许一个远程 listener；验证方式：实现完成，需真实 SSH 凭据验证（见 7.9）
- [x] 2.4 远程 `Listen` 失败（服务器拒绝 / 端口占用）写入 `task.lastErr` 并在列表可见；网络类错误退避重试、认证类错误停止重试（复用 `isAuthError`）；验证方式：实测启动失败时接口返回可读原因且规则保持未运行；`GatewayPorts` 相关分支待真实环境（见 7.6）
- [x] 2.5 重连后重建远程监听：`reconnect()` 成功后关闭旧 listener，由 supervisor 用新客户端重新 `Listen`；验证方式：实现完成，需真实 SSH 凭据验证（见 7.9）
- [x] 2.6 `RemoveForward` / `StopAll` 覆盖远程 listener：取消上下文 → 关闭 listener → 关闭活动连接 → 关闭客户端；验证方式：实现完成，需真实 SSH 凭据验证（见 7.8）
- [x] 2.7 并发与隔离沿用既有实现（每连接一个通道 + 双向 `io.Copy`，以规则 id 为任务键，允许 local/remote 使用相同端口号）；验证方式：实现完成，需真实 SSH 凭据验证

## 3. REST API（复用既有路由）

- [x] 3.1 规则创建/更新 handler 接受并规范化 `Direction`、`BindAddress`，非法方向返回 4xx；验证方式：实测 `Direction=bogus` 返回 400，省略方向创建出 `local`
- [x] 3.2 `PortForwardingResponse` 增加 `Direction`、`BindAddress`，`Status`/`LastError` 语义不变；验证方式：实测 `GET /api/portForwards` 返回两字段，缺省规则方向为 `local`
- [x] 3.3 「运行中拒绝修改」的范围扩展到方向、监听地址（运行判定改用规则 id）；验证方式：代码实现完成；实测「未运行规则改方向」正常返回 200（符合预期），拒绝分支待规则真正运行后验证（见 7.7）
- [x] 3.4 启停接口按方向工作，远程启动失败返回可读原因；验证方式：实测 `remote` 规则启动失败返回 500 且规则保持未运行，错误文案含 SSH 失败原因
- [x] 3.5 删除被引用 SSH 连接、凭据脱敏等既有约束不受影响；验证方式：相关代码路径未改动（上一变更已实测通过），本次实测列表响应仍无凭据明文

## 4. 前端页面（chaos-ui）

- [x] 4.1 `chaos-ui/src/utils/api.ts`：`PortForward` 增加 `Direction`、`BindAddress`；`PortForwardPayload` 增加同名可选字段（含 `PortForwardDirection` 类型）；验证方式：`pnpm build` 通过
- [x] 4.2 `chaos-ui/src/views/PortForward.vue` 规则弹窗新增「转发方向」单选（本地转发 `-L` / 远程转发 `-R`）与「监听地址」输入；验证方式：`pnpm build` 通过
- [x] 4.3 方向联动文案：`remote` 时端口标签为「远端监听端口」、目标提示「由本机侧解析」、监听地址占位提示 `127.0.0.1`；`local` 时为「本地监听端口 / 由服务器侧解析 / 0.0.0.0」；验证方式：`pnpm build` 通过，文案分支已实现（视觉确认见 7.13）
- [x] 4.4 规则列表：新增「方向」标签列，监听端口列表头为「监听端口」并在单元格内标注「本地端口 / 远端端口 + 监听地址」；验证方式：`pnpm build` 通过
- [x] 4.5 非回环监听地址的风险提示：`remote` + 非 `127.0.0.1`/`::1` 时提示「会把本机服务暴露给服务器网络」；验证方式：`pnpm build` 通过，提示逻辑已实现

## 5. 构建与整体验收

- [x] 5.1 `cd chaos-go && go build ./...` 与 `go vet ./internal/portfwd/...` 无错误；验证方式：命令输出为空，`gofmt -l` 无未格式化文件
- [x] 5.2 `cd chaos-ui && pnpm build` 通过，`dist` 已同步到 `chaos-go/cmd/server/web`（该目录已 gitignore），后端二进制重新构建成功；验证方式：`go build .\cmd\server\main.go` 成功
- [x] 5.3 按第 7 节「前端手动验证清单」逐项人工验收并记录结果；验证方式：用户已确认真实环境测试通过（含 7.4 远程数据面与 7.9 重连重建远端监听）

## 6. 后端测试（决策：本次不添加）

- [x] 6.1 用户已确认**本次不补充 `_test.go`**，沿用上一轮（`ssh-port-forward`）的决定，仅按第 7 节做人工验收。原因：远程转发的关键行为（服务器侧监听、`GatewayPorts` 限制、断线重建监听）必须在真实 sshd 上才能有效验证，mock 成本高、收益低。
- [x] 6.2 记录：本节为决策留痕，归档时保留；后续若需回归防护（如方向规范化、默认监听地址这类纯函数），再单独提一个补测试的 change。

## 7. 前端手动验证清单（用户人工验收）

- [x] 7.1 打开「端口转发」页面，既有（本次变更前创建的）规则正常展示，方向列显示为「本地 -L」，功能不回归
- [x] 7.2 新建一条「本地转发」规则（不填监听地址），启动后本机端口可访问 SSH 服务器内网目标（既有 `-L` 能力不回归）
- [x] 7.3 新建一条「远程转发」规则：远端端口 9000、目标 `127.0.0.1:3000`、监听地址留空，启动后状态为运行中
- [x] 7.4 在 SSH 服务器上执行 `curl http://127.0.0.1:9000` 能访问到本机 `3000` 服务（核心 `-R` 数据面）
- [x] 7.5 把远程规则监听地址改为 `0.0.0.0` 且服务器已开 `GatewayPorts`：从第三台机器可访问该端口
- [x] 7.6 服务器未开 `GatewayPorts` 时用 `0.0.0.0` 启动：页面提示可读失败原因，规则保持未运行，「最近错误」列显示该原因
- [x] 7.7 远程规则运行中修改方向 / 监听地址：页面提示需先停止
- [x] 7.8 停止远程规则：服务器侧连接该端口被拒绝；再次启动可恢复
- [x] 7.9 远程规则运行中断网再恢复：状态仍为运行中，服务器侧可重新访问（重连后重建监听）
- [x] 7.10 本地规则运行中断网再恢复：本机监听端口未释放，恢复后可继续转发
- [x] 7.11 远程规则与本地规则使用相同端口号：互不冲突，可同时运行
- [x] 7.12 删除一条运行中的远程规则：先停止再删除成功
- [x] 7.13 列表与弹窗展示：切换方向时端口标签、目标提示、监听地址占位与风险提示均正确变化
- [x] 7.14 重启后端后所有规则状态为未运行，方向与监听地址配置仍在
- [x] 7.15 全程检查后端日志：无 SSH 密码 / 私钥 / 口令明文

## 8. 实施与本地验证记录（AI）

- 代码落点：`chaos-go/internal/portfwd/{portfwd.go,forwarder.go}`（模型/校验/handler + 双向隧道引擎）、`chaos-go/sql/chaos_postgres_update.sql`、`chaos-ui/src/{views/PortForward.vue,utils/api.ts}`
- 关键结构变化：任务表由「端口号 → 任务」改为「规则 id → 任务」（本地/远程可能同端口）；`ForwardTask` 增加 `direction`/`bindAddr`/`listenAddr`，listener 改为可替换（`listenerMu`）；本地用 `localAcceptLoop` 持有稳定 listener，远程用 `remoteListenLoop` 监督重建；`reconnect()` 在远程方向额外关闭旧 listener 以触发重建
- 本地冒烟（临时目录 + 临时 SQLite + 端口 18082，已清理，未触碰真实库与服务）：
  - 省略 `Direction` 创建 → `Direction=local`、`BindAddress=0.0.0.0`、名称 `[L] 0.0.0.0:14001 → 127.0.0.1:3306`
  - `Direction=remote` 且省略 `BindAddress` → `BindAddress=127.0.0.1`、名称 `[R] 127.0.0.1:14002 → 127.0.0.1:3000`
  - `Direction=bogus` → 400「转发方向不合法，只能为 local 或 remote」
  - `GET /api/portForwards` 返回 `Direction`/`BindAddress`/`Status`/`LastError`
  - 启动 `remote` 规则（凭据错误）→ 500「SSH 连接失败…」且保持未运行
  - 未运行规则修改方向 → 200（符合「运行中才拒绝」的预期）
  - 日志凭据明文命中数：0
- 待用户在真实 SSH 环境验证：远程数据面（7.4）、`GatewayPorts` 分支（7.5/7.6）、重连重建远端监听（7.9）、运行中拒绝修改（7.7）、同端口双方向共存（7.11）
