## Why

现有 `internal/portfwd` 只实现了「本地端口 → 远程主机:端口」的裸 TCP 直连转发，既没有注册路由、也没有前端入口，且无法处理目标服务只在跳板机内网可达的场景。需要基于 SSH 隧道（本地转发，等价 `ssh -L`）实现端口转发，并把 SSH 连接信息（密码或私钥）持久化到数据库，做到一次录入、复用于多条转发规则、Web 界面一键启停。

## What Changes

- 在 `internal/portfwd` 中新增 SSH 隧道转发能力：本地监听端口 → SSH 服务器 → 远端目标 `host:port`，等价 `ssh -L <localPort>:<targetHost>:<targetPort> <user>@<sshHost>:<sshPort>`
- 新增 SSH 连接信息模型并持久化到数据库：SSH 主机 / 端口 / 用户名、认证方式（密码 / 私钥）、密码或私钥内容、可选私钥口令（passphrase）
- 扩展端口转发规则模型：关联某条 SSH 连接，声明本地监听端口、目标主机、目标端口、启用状态、备注
- 新增 REST 接口：SSH 连接增删改查、SSH 连接连通性测试、转发规则增删改查、转发启停
- 在 `routes.SetupRouter` 注册 `/api/sshConns`、`/api/portForwards` 路由组（统一 `/api` 前缀）
- 在 `cmd/server/main.go` 的 `AutoMigrate()` 登记新模型
- 前端 `chaos-ui` 新增「端口转发」页面：SSH 连接管理 + 转发规则管理 + 启停 + 运行状态展示
- **BREAKING**：现有未接线的裸 TCP 直连转发（`internal/portfwd` 的 `PortForwarding` 模型与 handler）被 SSH 隧道实现取代，转发语义由「直连目标」改为「经 SSH 隧道」

## Capabilities

### New Capabilities

- `ssh-port-forward`: SSH 隧道端口转发与 SSH 连接凭据管理，覆盖后端转发引擎、REST API、数据持久化与前端管理页面

### Modified Capabilities

<!-- 无既有 spec 需求变化：openspec/specs/ 下暂无端口转发相关能力 -->

## Impact

- `chaos-go/internal/portfwd/`：改造为 SSH 隧道转发，新增 SSH 连接建立、session 复用、隧道生命周期管理逻辑；沿用现有 `GlobalPortForwarder` 单例与启停/重试思路
- `chaos-go/internal/portfwd/portfwd.go`：`PortForwarding` 模型新增 SSH 连接关联与目标字段
- `chaos-go/routes/routes.go`：新增 `/api/sshConns`、`/api/portForwards` 路由注册
- `chaos-go/cmd/server/main.go`：`AutoMigrate()` 登记新增 / 变更的模型
- `chaos-go/sql/chaos_postgres_update.sql`：追加增量 DDL（新表 + 列变更，注明日期与用途）
- `chaos-go/go.mod`：`golang.org/x/crypto`（含 `x/crypto/ssh`）由间接依赖提升为直接依赖
- `chaos-ui/src/views/PortForward.vue`（新增）、`chaos-ui/src/router/index.ts`（新增菜单路由）、`chaos-ui/src/utils/api.ts`（新增接口封装）
- 数据库：新增 SSH 连接表；`port_forwarding` 表新增关联列与目标列
- 安全：数据库中将保存 SSH 密码 / 私钥，需在设计阶段明确存储与回显策略（明文 / 加密 / 脱敏），避免前端与日志泄露凭据
