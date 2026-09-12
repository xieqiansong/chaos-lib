## Why

现有 SSH 端口转发只支持本地转发（`ssh -L`）：在本机监听端口，把流量送到 SSH 服务器侧解析的目标。反向场景——让 **SSH 服务器（或其内网的其他机器）访问本机/本机内网的服务**（如临时把本地开发服务暴露给远程机器、让远程机器回调本地 webhook）——当前无法实现，只能手工敲 `ssh -R`。需要把远程转发（`ssh -R`）纳入同一套连接与规则管理，做到 Web 界面一键启停。

## What Changes

- 转发规则新增「方向」概念：`local`（本地转发，等价 `ssh -L`）与 `remote`（远程转发，等价 `ssh -R`），默认 `local` 保持既有行为不变
- 规则「监听端口」语义按方向泛化：`local` 时在本机监听；`remote` 时在 SSH 服务器侧监听
- 新增可选「监听地址（bind address）」：`local` 默认 `0.0.0.0`；`remote` 默认 `127.0.0.1`（受 SSH 服务器 `GatewayPorts` 限制，绑定非回环地址需服务器显式允许）
- 远程转发语义：SSH 服务器侧监听 → 流量经隧道回到本机 → 本机按「目标主机:目标端口」建立连接（目标由本机侧解析，与 `ssh -L` 的解析方向相反）
- 隧道引擎扩展为双向：远程转发时由 SSH 客户端在服务器侧申请监听（`ssh.Client.Listen`），并把每个接入连接回连到本机目标
- 隧道的保活与断线重连需覆盖远程监听：网络类断线重连后 MUST 重新在服务器侧建立监听
- 前端规则表单新增方向选择、监听地址输入；列表按方向展示「本地端口 / 远端端口」与「目标」含义提示
- 数据库：`port_forwarding` 表新增 `direction`、`bind_address` 列，存量数据视为 `local`
- 新增/扩展路由无变化（复用 `/api/portForwards`），仅请求/响应体增加字段

## Capabilities

### New Capabilities

<!-- 无新能力：远程转发是对既有 SSH 端口转发能力的扩展 -->

### Modified Capabilities

- `ssh-port-forward`: 规则模型由「本地监听」泛化为「按方向监听」，新增远程转发（`ssh -R`）能力。涉及 4 条既有需求的语义变更（Port forwarding rule management、Start and stop port forwarding、SSH tunnel forwarding behavior、Port forwarding management page），并新增远程转发专属需求

## Impact

- `chaos-go/internal/portfwd/portfwd.go`：`PortForwarding` 模型新增 `Direction`、`BindAddress`；规则校验按方向分支（`local` 校验本机端口可用性语义、`remote` 提示服务器侧限制）
- `chaos-go/internal/portfwd/forwarder.go`：`ForwardTask` 泛化为「监听源由方向决定」——`local` 用 `net.Listen`，`remote` 用 `ssh.Client.Listen`；重连逻辑需重建远程监听
- `chaos-go/internal/portfwd/portfwd.go` handlers：请求/响应 DTO 增加方向与监听地址；`toResponse` 的监听端口语义随方向变化
- `chaos-go/sql/chaos_postgres_update.sql`：追加增量 DDL（`direction`、`bind_address` 列 + 默认值，注明日期与用途）
- `chaos-ui/src/utils/api.ts`：`PortForward` 类型与创建/更新载荷增加 `Direction`、`BindAddress`
- `chaos-ui/src/views/PortForward.vue`：规则表单加方向单选与监听地址；列表列按方向动态展示；`SSH 连接` 区不变
- 复用既有 `ssh_connections` 表与凭据保护约束（不新增凭据字段、不改变脱敏要求）
- 运维提示：远程转发监听非回环地址需 SSH 服务器开启 `GatewayPorts`（或在客户端侧做好权限收敛），否则仅服务器本机可访问——需在 UI 上给出手册性提示
- 不需要新依赖（`golang.org/x/crypto/ssh` 已为直接依赖，`ssh.Client.Listen` 属既有 API）
