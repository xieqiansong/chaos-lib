## Context

见 `proposal.md - Why`。既有实现（已归档 `2026-09-12-ssh-port-forward`，真值源见 `openspec/specs/ssh-port-forward/spec.md`）与本次相关的现状：

- `internal/portfwd/portfwd.go` 的 `PortForwarding` 模型为「本地监听」单一句义：`Port`（本机监听端口）+ `TargetHost`/`TargetPort`（SSH 服务器侧解析）+ `SshConnectionId` + `Status` + `Remark`。
- `internal/portfwd/forwarder.go` 的 `ForwardTask` 持有 `listener net.Listener`（由 `net.Listen` 创建，一次创建、跨重连存活）与 `client *ssh.Client`；每来一个本地连接用 `client.Dial("tcp", targetAddr)` 开 `direct-tcpip` 通道后双向 `io.Copy`。
- `reconnect()` 只替换 `*ssh.Client`，本地 listener 不动 —— 这个不对称正是本次要泛化的点。
- 前端 `PortForward.vue` 的规则表单/表格只表达本地转发；`api.ts` 的 `PortForward` 类型无方向字段。
- 约束：GORM 模型不写 `column` 标签；模型需 `AutoMigrate` 登记；结构变动追加 `chaos_postgres_update.sql`；凭据保护要求（不回显、不写日志）不变。

## Goals / Non-Goals

**Goals:**

- 在**不新增表、不新增路由组**的前提下，把规则模型从「本地监听」泛化为「按方向监听」，同时支持 `ssh -L` 与 `ssh -R`。
- 存量规则（无方向）行为完全不变，默认按 `local` 处理。
- 远程转发支持可选监听地址，并在服务器拒绝绑定时给出可读错误。
- 远程转发在网络断线重连后能自动重建服务器侧监听。

**Non-Goals:**

- 不做动态转发（`ssh -D` / SOCKS）。
- 不做 ProxyJump / 多级跳板。
- 不做「同一端口在服务器侧占用」的本地预检（本机无法感知服务器端口占用，只能由服务器返回错误）。
- 不引入 `IsDeleted` 软删除（沿用既有硬删除）。
- 不改动 SSH 连接（凭据）相关的模型、接口与保护策略。

## Decisions

### D1: 用 `Direction` 枚举字段扩展同一张表，而不是新建表或新能力
`port_forwarding` 新增 `direction text`（取值 `local` / `remote`，空值视为 `local`）。
- 为什么：两个方向的字段集合几乎完全重合（监听端口、监听地址、目标、连接、状态、备注），差异只在「监听在哪一侧」。拆表会让规则列表、启停、引用校验全部翻倍。
- 备选：新建 `remote_forwarding` 表 → 前端两套 CRUD、连接删除引用校验要查两张表，收益低。
- 能力归属：这是 `ssh-port-forward` 能力的**语义扩展**，因此以该能力的 MODIFIED delta 表达，而不是新增 capability。

### D2: 监听来源抽象为「策略」，`ForwardTask` 按方向二选一
- `local`：`net.Listen("tcp", bindAddr:port)`，**一次创建、跨重连存活**（现状不变）。accept 循环每次连接现取 `task.getClient()`。
- `remote`：`sshClient.Listen("tcp", bindAddr:port)`（底层即 `tcpip-forward` 请求），**与客户端生命周期绑定**，必须在重连后重建。
- 为什么不用同一套「listener 失效就重建」：本地 listener 与 SSH 客户端无耦合，重建会短暂丢端口（连接被拒），是可见的行为退化。
- 备选：远程也提前建连一次本地 listener → 无意义（远程监听本来就在服务器侧）。

### D3: 远程侧监听用「监督循环 + 每次一个监听器」而非持有单个 listener
远程任务的 accept 循环按 `listener` 代际运行：外层 supervisor 拿到当前（可能已重连过的）客户端 → `Listen` → 内层 accept 循环跑到该 listener 结束（关闭/断线）→ 回到外层重新 `Listen`。
- 为什么：`ssh.Client.Listen` 返回的 listener 在客户端断开时即失效，无法像本地那样长期持有；代际化让「重连后重建监听」成为自然行为，而不是散落的补丁。
- 失败处理：`Listen` 返回的服务器拒绝错误（如 `GatewayPorts` 未开启、端口已被占用）写入 `task.lastErr` 并在列表中可见；若是网络类错误则按既有退避重试，认证类错误停止重试（沿用既有 `isAuthError` 判定）。
- 停止：`RemoveForward` 关闭客户端会连带使远程 listener 失效；同时显式关闭当前 listener 以尽快让 accept 循环退出。

### D4: 监听地址默认值与校验
- `local` 缺省 `0.0.0.0`（与现状一致：`net.Listen` 的 `:port`）。
- `remote` 缺省 `127.0.0.1`（与 `ssh -R port:host:hostport` 不带 bind_address 时的 OpenSSH 行为一致）。
- 校验：`BindAddress` 为空按方向取默认；非空时只做「是合法 host」的宽松校验（允许 IPv4 / IPv6 / 主机名），不做网段判断；绑定失败交由监听过程返回真实错误。

### D5: 运行状态与错误可见性沿用既有机制
列表的 `Status` 仍以内存中 `GlobalPortForwarder` 是否持有该规则为准，`LastError` 返回最近一次失败原因。远程任务在 `Listen` 失败时把服务器返回的原因写入 `LastError`，前端「最近错误」列直接可见（这是用户排查 `GatewayPorts` / 端口占用的主要入口）。

### D6: 字段与兼容策略
- 新增列：`direction text`、`bind_address text`；均允许为空。
- 读路径：`direction` 空 → `local`；`bind_address` 空 → 按方向取默认。存量行因此无需数据迁移。
- 写路径：创建/更新时把规范化后的值落库（空方向补 `local`）。
- SQLite 由 `AutoMigrate` 加列；PostgreSQL 在 `chaos_postgres_update.sql` 追加 `ADD COLUMN IF NOT EXISTS` + 默认值说明。

### D7: 前端按方向动态表达，避免两套表单
规则弹窗加「转发方向」单选（本地转发 `-L` / 远程转发 `-R`）+「监听地址」输入；切换方向时：
- 标签与占位文案变化（本地端口 ↔ 远端端口；目标提示「由服务器侧解析」↔「由本机侧解析」）
- 监听地址占位在 `remote` 时提示默认 `127.0.0.1`，并在旁给出「服务器需允许非回环绑定（GatewayPorts）」的说明
列表把「本地端口」列标题按行方向动态渲染为「本地端口」/「远端端口」。

### D8: 复用既有路由与凭据保护，不改 `/api/portForwards` 形状之外的东西
`GET/POST /api/portForwards`、`PATCH/DELETE /api/portForwards/:id`、`PATCH /api/portForwards/:id/status` 全部复用，仅在请求/响应体增加 `Direction`、`BindAddress`。`ssh_connections` 及凭据脱敏/日志约束完全不变。

## Risks / Trade-offs

- [远程监听受服务器策略限制，用户以为"坏了"] → 启动失败时把服务器原始错误透出到接口与页面「最近错误」列；表单在 `remote` 方向给出 GatewayPorts 说明。
- [远程监听绑定非回环地址可能把内网服务暴露出去，安全风险高] → 默认 `127.0.0.1`；界面对非回环地址给出风险提示文案；不在服务端强制拦截（用户自托管，需保留灵活性）。
- [重连期间远程监听短暂消失，服务器侧连接被拒] → 与 `ssh -R` 原生行为一致；重连成功后自动重建监听（D3），状态维持"运行中"，仅 `LastError` 记录期间错误。
- [断线重连与 accept 循环并发导致重复监听] → supervisor 串行化 `Listen`，同一时刻只持有一个 listener；`RemoveForward` 用取消上下文终止 supervisor。
- [方向字段引入后旧前端不传 direction] → 后端缺省补 `local`，保持向后兼容。
- [本地端口占用检测对远程方向不适用] → 明确 Non-Goal；由服务器错误承担提示职责，避免造假的预检。
- [spec 变更面较大（4 条 MODIFIED）] → 已在 delta 中逐条给出完整新内容与场景，避免归档时丢失既有场景。

## Migration Plan

1. 后端：模型加 `Direction`/`BindAddress` → `AutoMigrate` 自动加列（SQLite）+ 追加 PostgreSQL 增量 DDL。
2. 引擎：`ForwardTask` 按方向选择监听策略；远程任务引入 supervisor；重连后重建远程监听。
3. 接口：请求/响应体加字段，缺省规范化；无路由变更。
4. 前端：`api.ts` 类型 + `PortForward.vue` 表单/列表按方向动态。
5. 构建：`go build ./...`、`go vet`、`pnpm build`，`dist` 同步到 `cmd/server/web`。
6. 回滚：`direction` 空/`local` 即等价于本次变更前的行为；停止所有 `remote` 规则后，前端回退即可，无数据风险（新增列留空不影响旧代码读取）。
