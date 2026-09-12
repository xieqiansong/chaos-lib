## Context

见 `proposal.md - Why`。

现状与约束：

- `internal/portfwd` 已有 `PortForwarder` 单例（`GlobalPortForwarder`）、`PortForwarding` 模型与 4 个 handler，但**路由未注册**（`routes.SetupRouter` 中无 portfwd 相关注册），前端也没有任何入口，属于「写了没接线」的半成品。
- 现有转发是 `net.Listen` + `net.DialTimeout(targetAddr)` 的**裸 TCP 直连**，目标地址由本机解析，无法访问 SSH 服务器内网地址。
- `models/port_forwarding.go` 存在与 `internal/portfwd/portfwd.go` **重复且未使用**的模型定义。
- 项目硬规则：DB 用 `config.GetDB()` 单例；GORM 模型不写 `gorm:"column:..."`；模型需在 `cmd/server/main.go` 的 `AutoMigrate()` 登记；路由统一在 `routes.SetupRouter(webFS embed.FS)` 注册，前缀 `/api`；结构变动追加到 `chaos-go/sql/chaos_postgres_update.sql`。
- `golang.org/x/crypto v0.48.0` 已在 `go.mod`，但为 indirect；`golang.org/x/crypto/ssh` 可直接使用，无需新增第三方库。
- 数据库同时支持 SQLite（默认，单写连接）与 PostgreSQL。
- 前端 `chaos-ui`：菜单由 `src/router/index.ts` 的 `appRoutes` 自动生成，接口统一走 `src/utils/api.ts` 的 `sendMessage`。

## Goals / Non-Goals

**Goals:**

- 用 SSH 隧道（本地转发，等价 `ssh -L`）替换裸 TCP 直连转发，使远端目标由 SSH 服务器侧解析。
- SSH 连接信息（密码 / 私钥 + 可选口令）持久化到数据库，多条转发规则可复用同一连接。
- 转发规则可增删改查、可启停，运行状态与最近错误可见。
- 凭据不出现在 API 响应与日志中。
- 前端提供完整的端口转发管理页面。

**Non-Goals:**

- 不做远程转发（`ssh -R`）与动态转发（`ssh -D` / SOCKS 代理），本期只做本地转发（`ssh -L`）。
- 不做 SSH 跳板链（ProxyJump / 多级跳板）。
- 不做免密凭据的密钥库托管（直接存用户提供的私钥内容，不做系统 keychain 集成）。
- 不做端口转发的自动开机启动（重启后规则默认未运行）。

## Decisions

### D1: 隧道实现选用 `golang.org/x/crypto/ssh`，采用「每客户端连接一个 direct-tcpip channel」
用 `ssh.Client.Dial("tcp", targetHost:targetPort)`（底层即 `direct-tcpip` channel）建立到远端目标的通道，再与本地 accept 到的 `net.Conn` 做双向 `io.Copy`。

- 为什么：`ssh -L` 的语义正是「本机监听 → SSH 服务器按目标地址建 channel」，`ssh.Client.Dial` 即等价能力，无需自建 channel 协议。
- 备选：手写 `session` + `ssh.Dial` → 过度复杂；调用外部 `ssh.exe` 子进程 → 依赖系统环境、跨平台差、难管理，弃用。
- 复用现有 `ForwardTask` 的并发模型（`connections map[net.Conn]struct{}`、`activeConns`、`RemoveForward`/`StopAll`），只把「远端连接获取」由 `net.DialTimeout` 换成 `sshClient.Dial`。

### D2: 数据模型拆成两张表，`port_forwarding` 复用既有表名并加关联列
- `SshConnection`（表 `ssh_connections`）：`Id`、`Name`、`Host`、`Port`(默认 22)、`Username`、`AuthType`(`password`/`key`)、`Password`、`PrivateKey`、`Passphrase`、`Remark`。
- `PortForwarding`（表 `port_forwarding`，复用）：新增 `SshConnectionId`、`Remark`；保留 `Name`、`Port`(本地监听)、`TargetHost`、`TargetPort`、`Status`。
- 为什么不合并成一张表：连接信息（凭据）与转发规则是「一对多」，分离后改主机名/改密码只需改一处；也避免列表接口反复带出凭据列。
- 软删除：本期沿用现有 `filelink` / `portfwd` 的**硬删除**行为（`Delete`），不引入 `IsDeleted`；连接被规则引用时拒绝删除（见 spec），无需回收站。
- 删除 `models/port_forwarding.go` 的重复未使用模型，模型与 handler 同包（遵循既有组织方式）。

### D3: 凭据存储采用「本地库明文 + 强不暴露」策略
自托管单用户工具，库文件在本机，凭据不做应用层加密；代价可控，换取实现简单与「密码错就密码错」的清晰排障。防护措施：

- 响应模型与 GORM 模型分离：列表 / 详情接口返回专用 DTO，凭据字段一律替换为 `HasCredential bool`（或 `HasPassword` / `HasPrivateKey`）。
- 更新接口中凭据字段为空即「保持原值」，避免前端回显。
- 所有 `slog` 调用只记录 `host` / `port` / `username` / 错误摘要，绝不打印 `Password` / `PrivateKey` / `Passphrase`。
- 备选：AES-GCM 加密存储（密钥来自环境变量）→ 需新增密钥管理、密钥轮换与迁移，本期不做；作为后续可选项记录在 Risks。

### D4: Host Key 校验采用 `ssh.InsecureIgnoreHostKey`，并显式记录该权衡
首次连接不校验服务器指纹，便于快速接入；测试接口返回 SSH 服务器标识，便于用户自行比对。

- 备选：`known_hosts` 校验 + 指纹确认弹窗 → 交互与持久化复杂（前端需多一次确认流程），本期不做；记录为后续增强。

### D5: 连接生命周期与保活
- 每条规则持有 1 个 `*ssh.Client` + 1 个本地 `net.Listener`；启动时「先认证建连，再监听端口」——认证失败则不占用端口。
- 保活：定时（如 30s）通过 `sshClient.SendRequest("keepalive@openssh.com", true, nil)` 探测；失败即触发重连。
- 断线重连：沿用现有 `startRetryMonitor` + `retryTicker`（15s）思路，隧道断开时重建 `ssh.Client`；重建期间本地监听保持，新连接会短暂失败并重试（沿用 `connectWithRetry` 语义）。
- 停止 / 删除：关闭 listener → 关闭活动连接 → 关闭 `ssh.Client`（顺序保证 accept 循环尽快退出）。
- 全局单例 `GlobalPortForwarder` 保持，`StopAll` 在进程退出时调用（如已有退出钩子则接入）。

### D6: 路由与接口
```
GET    /api/sshConns              列表（凭据脱敏）
POST   /api/sshConns              新建
PATCH  /api/sshConns/:id          更新（凭据留空=保持原值）
DELETE /api/sshConns/:id          删除（被引用则拒绝）
POST   /api/sshConns/:id/test     连通性测试

GET    /api/portForwards          列表（含运行状态与最近错误）
POST   /api/portForwards          新建
PATCH  /api/portForwards/:id      更新（运行中拒绝）
DELETE /api/portForwards/:id      删除（运行中先停）
PATCH  /api/portForwards/:id/status   启停 {status: bool}
```
命名沿用现有复数资源风格（`fileLinks` / `sdks` / `quickEdits`）与 `/api` 前缀。

### D7: 前端新增独立页面 `PortForward.vue`
- `chaos-ui/src/views/PortForward.vue`：上区 SSH 连接管理（表格 + 新建/编辑弹窗 + 测试按钮），下区转发规则管理（表格 + 新建/编辑弹窗 + 启停开关 + 状态/错误列）。
- `src/router/index.ts` 追加路由 `{path:'/portForward', meta:{title:'端口转发', icon:'Connection'}}`，菜单自动出现。
- `src/utils/api.ts` 追加类型与封装函数（`getSshConns` / `createSshConn` / `updateSshConn` / `deleteSshConn` / `testSshConn` / `getPortForwards` / ...），沿用 `sendMessage`。
- 表单中凭据字段：编辑时留空，placeholder 提示「留空表示不修改」。

### D8: 运行状态以内存为准，进程重启后回归「未运行」
进程重启后内存中没有任何隧道，因此不在启动阶段自动建连（避免用错误凭据反复重试刷日志）。列表接口以 `GlobalPortForwarder` 是否实际持有该本地端口来判定运行状态，DB 中的 `Status` 仅作落库兜底；服务启动时将所有规则 `Status` 归零，保持「内存实际状态」与「展示状态」一致。

## Risks / Trade-offs

- [凭据以明文落库] → 加强不暴露约束（DTO 脱敏 + 日志脱敏 + 更新留空即保留）；后续可加 AES-GCM 加密与密钥管理（非本期）。
- [`InsecureIgnoreHostKey` 存在中间人风险] → 仅用于自托管可信网络；测试接口返回服务器标识供人工核对；后续可加指纹信任流程。
- [凭据错误时若自动重连会反复认证失败刷日志] → 认证类错误不进入自动重连，只记录一次并保持规则未运行；仅网络类错误重试。
- [本地端口与既有 `portfwd` 数据冲突] → 同一本地端口在内存单例中唯一；`AddForward` 对已存在端口返回明确错误而非静默跳过（现有实现是「跳过并返回 nil」，需改为报错以符合 spec）。
- [删除连接被规则引用] → 先查 `port_forwarding` 中未删除且引用该连接的规则数，>0 则拒绝。
- [PostgreSQL / SQLite 双库字段类型差异] → 仅用基础类型（string/int/bool），不引入 JSON 列，避免方言差异；增量 DDL 只针对 PostgreSQL 快照补充，SQLite 由 AutoMigrate 负责。
- [前端无自动化测试] → 按 tasks.md 的「前端手动验证清单」人工验收。

## Migration Plan

1. 后端：新增 / 改造模型与 handler → `AutoMigrate` 登记 → 追加 `chaos_postgres_update.sql` 增量 DDL。
2. 网关：`routes.SetupRouter` 注册两组路由；`go build ./...` 通过。
3. 前端：新增页面与路由、接口封装 → `pnpm build` → 后端重新构建以嵌入新前端。
4. 回滚：撤销路由注册即恢复「功能不可见」；新增表与列不影响既有表，无需数据回滚（如需彻底回滚再执行对应 `DROP` / `DROP COLUMN`，由人工执行）。
