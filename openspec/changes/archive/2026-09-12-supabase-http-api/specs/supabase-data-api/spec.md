## Purpose

为 `chaos-lib` 提供一条不依赖 Postgres 线协议、可在 IPv4-only 网络下工作的云端数据读写通道：通过 Supabase Data API（PostgREST）对明确划归云端的表执行增删改查，并约束凭据的使用方式与失败语义，使该通道可在不影响既有本地数据体系的前提下独立启用或停用。

## ADDED Requirements

### Requirement: Cloud data channel configuration

系统 SHALL 从配置中读取云端数据通道所需的项目地址、后端凭据、目标 schema 与请求超时；未提供项目地址或后端凭据时，该通道 MUST 被视为不可用，MUST NOT 发起任何外部请求，且 MUST NOT 影响其余功能的启动与运行。后端凭据 MUST NOT 出现在任何被版本控制的文件、日志或返回给调用方的错误文本中。

#### Scenario: Channel enabled by complete configuration

- **WHEN** 配置中同时提供了项目地址与后端凭据
- **THEN** 系统将该通道视为可用

#### Scenario: Channel disabled by missing configuration

- **WHEN** 配置中缺少项目地址或后端凭据
- **THEN** 系统将该通道视为不可用、不发起任何外部请求，且其余功能正常可用

#### Scenario: Credentials never exposed

- **WHEN** 系统输出日志或向调用方返回错误
- **THEN** 输出内容中不包含后端凭据的任何片段

#### Scenario: Request timeout is bounded

- **WHEN** 云端请求超过配置的超时时间仍未返回
- **THEN** 系统中止该请求并返回可读的超时错误，MUST NOT 无限等待

### Requirement: Credential transmission rule

系统 SHALL 仅通过 `apikey` 请求头传递凭据。系统 MUST NOT 把凭据放入 `Authorization: Bearer` 请求头，因为 Supabase 新式凭据（`sb_publishable_*` / `sb_secret_*`）不是 JWT，放入该头会被平台拒绝。后端读写 MUST 使用绕过行级安全性（RLS）的 secret 凭据；受 RLS 约束的 publishable 凭据 MUST NOT 被用于写入。

#### Scenario: Credential sent on the dedicated header only

- **WHEN** 系统向云端发起任意一次请求
- **THEN** 该请求携带 `apikey` 请求头且 `Authorization` 请求头不携带该凭据

#### Scenario: Write with a publishable credential

- **WHEN** 使用受 RLS 约束的 publishable 凭据执行写入
- **THEN** 系统把平台返回的权限拒绝错误作为失败结果返回，MUST NOT 静默视为成功

### Requirement: Reading rows from a cloud table

系统 SHALL 支持读取指定云端表的行，并 SHALL 支持下列约束的组合：列选择、等值与范围过滤、排序、返回条数上限。系统 MUST 仅对显式允许的表名发起请求；表名 MUST NOT 直接来自未经校验的外部输入。

#### Scenario: Read rows with filters

- **WHEN** 请求读取某张允许的表，并指定过滤条件、排序与条数上限
- **THEN** 系统返回满足过滤条件、按指定顺序排列且不超过条数上限的行

#### Scenario: Read from an empty result set

- **WHEN** 请求读取的行不存在或过滤条件未匹配到任何行
- **THEN** 系统返回空结果集且 MUST NOT 视为错误

#### Scenario: Read from a non-existent table

- **WHEN** 请求读取的表不存在
- **THEN** 系统返回可读的失败原因，并区分「表不存在」与「无权限访问」

#### Scenario: Table name not allowlisted

- **WHEN** 请求使用了未在允许清单中的表名
- **THEN** 系统拒绝该请求且 MUST NOT 向云端发出任何请求

### Requirement: Creating rows in a cloud table

系统 SHALL 支持向指定云端表插入单行或多行。系统 SHALL 支持在冲突列上执行 upsert（存在则更新、不存在则插入）。当调用方需要写入后的结果时，系统 SHALL 返回写入后的行；否则不得要求调用方额外发起一次读取来获取该结果。

#### Scenario: Insert a single row

- **WHEN** 向允许的表提交一行数据，且调用方要求返回写入结果
- **THEN** 系统返回该行写入后的内容，并包含由数据库生成的字段

#### Scenario: Insert multiple rows

- **WHEN** 向允许的表提交多行数据
- **THEN** 系统以一次请求插入全部行，并返回写入后的全部行

#### Scenario: Upsert on conflict

- **WHEN** 向允许的表提交数据并指定冲突列，且该列已存在相同值
- **THEN** 系统更新该行而非报错，并返回更新后的行

#### Scenario: Insert violates a database constraint

- **WHEN** 提交的数据违反非空、唯一或外键约束
- **THEN** 系统返回失败，且错误内容可定位到违反的约束

### Requirement: Updating rows in a cloud table

系统 SHALL 支持按过滤条件更新指定云端表中匹配的行，并 SHALL 在调用方需要时返回更新后的行。

#### Scenario: Update matched rows

- **WHEN** 按过滤条件更新允许的表中的行，且调用方要求返回更新结果
- **THEN** 系统返回更新后的行，且未匹配的行不被修改

#### Scenario: Update matches no rows

- **WHEN** 更新条件未匹配到任何行
- **THEN** 系统返回空结果集且 MUST NOT 视为错误

#### Scenario: Update without a filter condition

- **WHEN** 更新请求未携带任何过滤条件
- **THEN** 系统拒绝该请求，以避免整表被改写

### Requirement: Deleting rows from a cloud table

系统 SHALL 支持按过滤条件删除指定云端表中匹配的行。删除被行级安全性（RLS）或其他策略拦截时，系统 SHALL 返回失败而非静默成功。

#### Scenario: Delete matched rows

- **WHEN** 按过滤条件删除允许的表中的行
- **THEN** 系统删除匹配的行并返回被删除行的内容

#### Scenario: Delete without a filter condition

- **WHEN** 删除请求未携带任何过滤条件
- **THEN** 系统拒绝该请求，以避免整表被清空

#### Scenario: Delete blocked by a policy

- **WHEN** 删除操作被行级安全性或其他策略拦截
- **THEN** 系统返回失败并给出可读原因，MUST NOT 返回成功

### Requirement: Failure semantics of the cloud data channel

系统 SHALL 把云端返回的非成功状态码转换为包含状态码与原始响应内容的可读错误；MUST NOT 把失败静默当作成功返回空结果。系统 SHALL 区分「网络不可达 / 超时」与「服务端返回错误」，以便调用方决定是否重试。

#### Scenario: Server returns an error status

- **WHEN** 云端返回 4xx 或 5xx 状态码
- **THEN** 系统返回包含该状态码与服务端原始信息的错误，且不返回成功结果

#### Scenario: Network unreachable

- **WHEN** 云端地址不可达或 TLS 握手失败
- **THEN** 系统返回区分于服务端错误的可读失败原因

#### Scenario: Free-tier project is suspended

- **WHEN** 因免费套餐项目处于休眠状态导致首次请求失败
- **THEN** 系统返回可读失败原因，且后续重试可成功时正常返回结果
