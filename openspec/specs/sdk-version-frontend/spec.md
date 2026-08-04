# sdk-version-frontend Specification

## Purpose
在已落地的后端 SDK 来源入库能力（`sdk-version-db` 变更）之上，补齐前端：让每个 SDK 类型的来源配置（repo/single 与路径）**可见**，让来源/类型**可管理**（增删改），并**正确区分**可切换与不可切换的来源，避免对 single 来源误触发切换。
## Requirements
### Requirement: 前端展示每个 SDK 类型的来源数组
系统 SHALL 在 SDK 版本管理视图中，对 `GET /api/sdks/defs` 返回的每条 SDK 类型，展示其 `sources` 数组，其中每个元素以「repo / single」标识区分并展示其 `root` 绝对路径。

#### Scenario: 展示混合来源
- **WHEN** 某 SDK 类型的 `sources` 同时含 repo 与 single 元素
- **THEN** 前端分别展示两类来源，并各自标注 kind 与 root 路径

#### Scenario: 来源信息读取失败降级
- **WHEN** `GET /api/sdks/defs` 请求失败
- **THEN** 版本展示区仍按 `GET /api/sdks` 正常渲染，不阻塞只读视图

### Requirement: 仅仓库来源可触发切换
系统 SHALL 仅当 SDK 类型含有至少一个 `repo` 来源时，允许对其版本标签触发切换；若类型仅含 `single` 来源，对应版本标签不可点击切换，并在误触发时提示「单版本来源不可切换」。

#### Scenario: 仅 single 来源禁用切换
- **WHEN** 某 SDK 类型 `sources` 全部为 single
- **THEN** 其版本标签不可点击，点击无效

#### Scenario: 含 repo 来源允许切换
- **WHEN** 某 SDK 类型 `sources` 含至少一个 repo
- **THEN** 其版本标签可点击，调用 `PATCH /api/sdks/:type/switch` 切换

#### Scenario: 切换被后端拒绝时提示
- **WHEN** 调用切换接口返回 400（单版本不支持切换）
- **THEN** 前端以消息提示「该来源不支持切换」，不改变本地状态

### Requirement: 前端可管理 SDK 类型与来源
系统 SHALL 提供管理界面，通过 `GET/POST /api/sdks/defs`、`PATCH/DELETE /api/sdks/defs/:name` 对 SDK 类型及其 `sources` 数组进行新增、编辑、软删除，并在操作后刷新视图。

#### Scenario: 新增 SDK 类型
- **WHEN** 在管理界面提交 Name 与 sources（含 kind/root）
- **THEN** 调用 `POST /api/sdks/defs`，成功后刷新 defs 与版本列表

#### Scenario: 编辑来源数组
- **WHEN** 对某类型编辑其 sources（增删 repo/single 项或修改 root）
- **THEN** 调用 `PATCH /api/sdks/defs/:name`，成功后视图更新

#### Scenario: 删除 SDK 类型
- **WHEN** 在管理界面对某类型确认删除
- **THEN** 调用 `DELETE /api/sdks/defs/:name`，成功后该类型从列表移除

