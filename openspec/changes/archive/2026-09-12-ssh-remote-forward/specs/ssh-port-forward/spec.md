## MODIFIED Requirements

### Requirement: Port forwarding rule management
系统 SHALL 支持端口转发规则的创建、查询、更新与删除。每条规则 MUST 关联一条已存在的 SSH 连接，并声明转发方向（`local` 本地转发 / `remote` 远程转发，缺省 `local`）、监听端口、可选监听地址、目标主机、目标端口、备注与启用状态。监听端口的含义随方向变化：`local` 时为本机监听端口，`remote` 时为 SSH 服务器侧监听端口。

#### Scenario: Create a rule bound to a connection
- **WHEN** 提交一条引用已存在 SSH 连接、未指定方向的规则，且监听端口与目标端口均在 1-65535 之间
- **THEN** 系统按 `local` 方向持久化该规则，返回其 id，初始状态为未启动

#### Scenario: Create a remote rule
- **WHEN** 提交一条方向为 `remote`、含监听端口与目标主机:目标端口的规则
- **THEN** 系统持久化该规则并返回其 id，初始状态为未启动

#### Scenario: Rule referencing unknown connection rejected
- **WHEN** 提交的规则引用了不存在的 SSH 连接
- **THEN** 系统拒绝并提示 SSH 连接不存在

#### Scenario: Invalid port rejected
- **WHEN** 提交的规则监听端口或目标端口不在 1-65535 之间，或目标主机为空
- **THEN** 系统拒绝并返回校验错误

#### Scenario: Invalid direction rejected
- **WHEN** 提交的规则方向不是 `local` 或 `remote`
- **THEN** 系统拒绝并返回校验错误

#### Scenario: Modify a running rule
- **WHEN** 修改一条处于运行状态规则的方向、SSH 连接、监听端口、监听地址或目标
- **THEN** 系统拒绝修改并提示需先停止该转发

#### Scenario: Delete a rule
- **WHEN** 删除一条转发规则
- **THEN** 若该规则正在运行系统先停止其转发，再删除记录

#### Scenario: Legacy rule without direction
- **WHEN** 查询一条在本次变更前创建、未记录方向的规则
- **THEN** 系统按 `local` 方向对待并展示

### Requirement: Start and stop port forwarding
系统 SHALL 支持启动与停止单条转发规则。启动时 MUST 经其关联的 SSH 连接建立隧道，并按方向完成监听（`local` 在本机监听，`remote` 在 SSH 服务器侧监听）；停止时 MUST 释放该监听并关闭隧道。

#### Scenario: Start a rule
- **WHEN** 对未启动的 `local` 规则执行启动
- **THEN** 系统建立 SSH 隧道、在本机监听指定端口，并把规则状态更新为运行中

#### Scenario: Start a remote rule
- **WHEN** 对未启动的 `remote` 规则执行启动
- **THEN** 系统建立 SSH 隧道、在 SSH 服务器侧建立指定监听，并把规则状态更新为运行中

#### Scenario: Duplicate start rejected
- **WHEN** 对已处于运行状态的规则再次执行启动
- **THEN** 系统拒绝并提示该转发已启动

#### Scenario: Local port already in use
- **WHEN** 启动 `local` 规则时本机监听端口已被其他进程占用
- **THEN** 系统启动失败、保持规则未运行，并返回端口占用错误

#### Scenario: Remote listen rejected by server
- **WHEN** 启动 `remote` 规则时 SSH 服务器拒绝该监听（端口被占用或不允许绑定该地址）
- **THEN** 系统启动失败、保持规则未运行，并返回可读的失败原因

#### Scenario: SSH authentication failure on start
- **WHEN** 启动时 SSH 主机不可达或凭据认证失败
- **THEN** 系统启动失败、保持规则未运行，并返回可读的失败原因

#### Scenario: Stop a rule
- **WHEN** 对运行中的规则执行停止
- **THEN** 系统释放监听、关闭隧道、断开该规则的活动连接，并把状态更新为未运行

#### Scenario: Stop an idle rule rejected
- **WHEN** 对未运行的规则执行停止
- **THEN** 系统拒绝并提示该转发未启动

### Requirement: SSH tunnel forwarding behavior
系统 SHALL 在监听端口与目标地址之间经 SSH 通道做双向数据转发。目标地址的解析侧 MUST 随方向变化：`local` 由 SSH 服务器侧解析（等价 `ssh -L`），`remote` 由本机侧解析（等价 `ssh -R`）。

#### Scenario: Traffic reaches remote-only target
- **WHEN** `local` 规则的远端目标地址（如 `127.0.0.1:3306`）仅在 SSH 服务器内网可达，本机客户端连接本机监听端口
- **THEN** 数据经 SSH 隧道转发至远端目标，客户端可正常收发

#### Scenario: Concurrent connections isolated
- **WHEN** 多个客户端同时连接同一监听端口
- **THEN** 每条连接独立转发，互不干扰

#### Scenario: Connection closed on stop
- **WHEN** 停止规则或隧道断开
- **THEN** 该规则关联的活动连接被关闭

#### Scenario: Status is observable
- **WHEN** 查询转发规则列表
- **THEN** 每条规则返回其方向、监听端口、运行状态与最近一次错误信息（如有）

### Requirement: Port forwarding management page
系统 SHALL 在 Web 前端提供端口转发管理页面，用于维护 SSH 连接与转发规则（含转发方向与监听地址）、启停转发并查看运行状态。

#### Scenario: Page lists connections and rules
- **WHEN** 打开端口转发页面
- **THEN** 页面分别展示 SSH 连接列表与转发规则列表及其方向、监听端口与运行状态

#### Scenario: Create and edit from the page
- **WHEN** 用户在页面上新增或编辑 SSH 连接 / 转发规则（含方向与监听地址）并保存
- **THEN** 页面调用对应接口并在成功后刷新列表

#### Scenario: Direction-aware labels
- **WHEN** 规则方向为 `remote`
- **THEN** 页面把监听端口标注为「远端端口」并提示该监听位于 SSH 服务器侧，`local` 时标注为「本地端口」

#### Scenario: Toggle forwarding from the page
- **WHEN** 用户在规则行上点击启动 / 停止
- **THEN** 页面调用启停接口，并按结果更新该行状态，失败时提示错误

#### Scenario: Credentials masked in the form
- **WHEN** 用户编辑一条已有 SSH 连接
- **THEN** 密码 / 私钥输入框不回显明文，留空表示保持原凭据

## ADDED Requirements

### Requirement: Remote port forwarding
系统 SHALL 支持远程转发：在 SSH 服务器侧按规则的监听地址与监听端口建立监听，并把接入的连接经隧道回连到本机侧解析的目标主机:目标端口。监听地址缺省为 `127.0.0.1`；绑定非回环地址 MUST 由 SSH 服务器策略（如 `GatewayPorts`）允许，否则启动失败并返回可读原因。隧道因网络原因重连后，系统 MUST 重新在服务器侧建立该监听。

#### Scenario: Expose a local service to the SSH server
- **WHEN** 一条 `remote` 规则监听服务器侧 `127.0.0.1:9000`、目标为本机 `127.0.0.1:3000`，且规则已启动
- **THEN** 在 SSH 服务器上连接 `127.0.0.1:9000` 的流量经隧道到达本机 `127.0.0.1:3000`

#### Scenario: Bind to non-loopback address
- **WHEN** 规则的监听地址设为 `0.0.0.0` 且 SSH 服务器允许非回环绑定
- **THEN** 系统在服务器侧按该地址建立监听，服务器外部可达

#### Scenario: Server refuses non-loopback bind
- **WHEN** 规则监听地址为 `0.0.0.0` 但 SSH 服务器未允许非回环绑定（未开启 `GatewayPorts`）
- **THEN** 系统启动失败、保持规则未运行，并返回可读的失败原因

#### Scenario: Reconnect re-establishes the remote listener
- **WHEN** 运行中的 `remote` 规则因网络原因断线后重连成功
- **THEN** 系统在 SSH 服务器侧重新建立该监听，规则仍显示为运行中

#### Scenario: Stop releases the remote listener
- **WHEN** 停止一条运行中的 `remote` 规则
- **THEN** 服务器侧监听被释放，之后服务器上连接该端口不再到达本机目标
