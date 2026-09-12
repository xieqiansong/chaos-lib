## Purpose

让用户通过 Web 界面维护 SSH 连接信息（密码或私钥）与端口转发规则，并一键建立经 SSH 隧道的本地端口转发，从而访问只有 SSH 服务器内网可达的服务。

## ADDED Requirements

### Requirement: SSH connection management
系统 SHALL 支持对 SSH 连接信息进行创建、查询、更新与删除。单条 SSH 连接至少包含：名称、SSH 主机、SSH 端口（默认 22）、用户名、认证方式。

#### Scenario: Create a password-auth connection
- **WHEN** 提交一条认证方式为「密码」的 SSH 连接（含名称、主机、端口、用户名、密码）
- **THEN** 系统持久化该连接并返回其 id

#### Scenario: Create a key-auth connection
- **WHEN** 提交一条认证方式为「私钥」的 SSH 连接（含私钥内容，可选私钥口令）
- **THEN** 系统持久化该连接并返回其 id

#### Scenario: Missing required fields rejected
- **WHEN** 提交的 SSH 连接缺少主机、用户名，或端口不在 1-65535 之间
- **THEN** 系统拒绝并返回明确的校验错误

#### Scenario: Update a connection
- **WHEN** 更新某条已存在连接的名称、主机或用户名
- **THEN** 系统保存变更，后续使用该连接的转发规则采用新配置

#### Scenario: Delete a connection in use is blocked
- **WHEN** 删除一条仍被任意转发规则引用的 SSH 连接
- **THEN** 系统拒绝删除并提示被引用的规则数量

### Requirement: SSH credential persistence
系统 SHALL 持久化 SSH 认证凭据，支持密码与私钥两种认证方式；私钥方式 MUST 支持可选的私钥口令（passphrase）。

#### Scenario: Password authentication is used for the tunnel
- **WHEN** 一条使用密码认证的连接被用于启动转发
- **THEN** 系统以保存的密码完成 SSH 认证

#### Scenario: Private key authentication is used for the tunnel
- **WHEN** 一条使用私钥认证的连接被用于启动转发
- **THEN** 系统以保存的私钥（及可选口令）完成 SSH 认证

#### Scenario: Switch authentication method
- **WHEN** 将一条连接从密码认证改为私钥认证并保存
- **THEN** 后续认证仅使用私钥，且不再要求原密码

### Requirement: Credential protection
系统 SHALL NOT 在列表与详情接口的响应中返回 SSH 密码或私钥明文，且 MUST NOT 将密码、私钥及其口令写入日志。

#### Scenario: Credentials not echoed by list API
- **WHEN** 请求 SSH 连接列表或详情
- **THEN** 响应中不含密码 / 私钥明文，仅可包含「是否已配置凭据」等布尔标记

#### Scenario: Credentials not written to logs
- **WHEN** 建立 SSH 连接失败或转发隧道异常
- **THEN** 日志中只记录主机、端口、用户名与错误摘要，不含密码或私钥内容

#### Scenario: Update without re-entering credential
- **WHEN** 更新连接的名称 / 主机等非凭据字段且未提交密码或私钥
- **THEN** 系统保留原凭据不变

### Requirement: Port forwarding rule management
系统 SHALL 支持端口转发规则的创建、查询、更新与删除。每条规则 MUST 关联一条已存在的 SSH 连接，并声明本地监听端口、目标主机、目标端口、备注与启用状态。

#### Scenario: Create a rule bound to a connection
- **WHEN** 提交一条引用已存在 SSH 连接、且本地端口与目标端口均在 1-65535 之间的规则
- **THEN** 系统持久化该规则并返回其 id，初始状态为未启动

#### Scenario: Rule referencing unknown connection rejected
- **WHEN** 提交的规则引用了不存在的 SSH 连接
- **THEN** 系统拒绝并提示 SSH 连接不存在

#### Scenario: Invalid port rejected
- **WHEN** 提交的规则本地端口或目标端口不在 1-65535 之间，或目标主机为空
- **THEN** 系统拒绝并返回校验错误

#### Scenario: Modify a running rule
- **WHEN** 修改一条处于运行状态规则的 SSH 连接、端口或目标
- **THEN** 系统拒绝修改并提示需先停止该转发

#### Scenario: Delete a rule
- **WHEN** 删除一条转发规则
- **THEN** 若该规则正在运行系统先停止其转发，再删除记录

### Requirement: Start and stop port forwarding
系统 SHALL 支持启动与停止单条转发规则。启动时 MUST 经其关联的 SSH 连接建立隧道，并在本地监听指定端口；停止时 MUST 释放本地监听端口并关闭隧道。

#### Scenario: Start a rule
- **WHEN** 对未启动的规则执行启动
- **THEN** 系统建立 SSH 隧道、监听本地端口，并把规则状态更新为运行中

#### Scenario: Duplicate start rejected
- **WHEN** 对已处于运行状态的规则再次执行启动
- **THEN** 系统拒绝并提示该转发已启动

#### Scenario: Local port already in use
- **WHEN** 启动时本地监听端口已被其他进程占用
- **THEN** 系统启动失败、保持规则未运行，并返回端口占用错误

#### Scenario: SSH authentication failure on start
- **WHEN** 启动时 SSH 主机不可达或凭据认证失败
- **THEN** 系统启动失败、保持规则未运行，并返回可读的失败原因

#### Scenario: Stop a rule
- **WHEN** 对运行中的规则执行停止
- **THEN** 系统关闭监听与隧道、断开该规则的活动连接，并把状态更新为未运行

#### Scenario: Stop an idle rule rejected
- **WHEN** 对未运行的规则执行停止
- **THEN** 系统拒绝并提示该转发未启动

### Requirement: SSH tunnel forwarding behavior
系统 SHALL 在本地监听端口与远端目标之间经 SSH 通道做双向数据转发；远端目标地址 MUST 由 SSH 服务器侧解析（等价 `ssh -L`），本地不与远端目标直连。

#### Scenario: Traffic reaches remote-only target
- **WHEN** 远端目标地址（如 `127.0.0.1:3306`）仅在 SSH 服务器内网可达，本地客户端连接本地监听端口
- **THEN** 数据经 SSH 隧道转发至远端目标，客户端可正常收发

#### Scenario: Concurrent connections isolated
- **WHEN** 多个客户端同时连接同一本地监听端口
- **THEN** 每条连接独立转发，互不干扰

#### Scenario: Connection closed on stop
- **WHEN** 停止规则或隧道断开
- **THEN** 该规则关联的活动连接被关闭

#### Scenario: Status is observable
- **WHEN** 查询转发规则列表
- **THEN** 每条规则返回其运行状态与最近一次错误信息（如有）

### Requirement: SSH connectivity test
系统 SHALL 提供 SSH 连接测试能力，在不启动端口转发的前提下验证主机可达性与凭据有效性。

#### Scenario: Test succeeds
- **WHEN** 对一条配置正确、主机可达的连接发起测试
- **THEN** 系统返回成功及 SSH 服务器标识信息

#### Scenario: Test fails with reason
- **WHEN** 对主机不可达、端口错误或凭据错误的连接发起测试
- **THEN** 系统返回失败及可读原因（如认证失败、连接超时）

### Requirement: Port forwarding management page
系统 SHALL 在 Web 前端提供端口转发管理页面，用于维护 SSH 连接与转发规则、启停转发并查看运行状态。

#### Scenario: Page lists connections and rules
- **WHEN** 打开端口转发页面
- **THEN** 页面分别展示 SSH 连接列表与转发规则列表及其运行状态

#### Scenario: Create and edit from the page
- **WHEN** 用户在页面上新增或编辑 SSH 连接 / 转发规则并保存
- **THEN** 页面调用对应接口并在成功后刷新列表

#### Scenario: Toggle forwarding from the page
- **WHEN** 用户在规则行上点击启动 / 停止
- **THEN** 页面调用启停接口，并按结果更新该行状态，失败时提示错误

#### Scenario: Credentials masked in the form
- **WHEN** 用户编辑一条已有 SSH 连接
- **THEN** 密码 / 私钥输入框不回显明文，留空表示保持原凭据
