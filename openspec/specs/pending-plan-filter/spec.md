# pending-plan-filter Specification

## Purpose
在「待办任务」页提供按任务计划树收敛列表的能力：用户点击工具栏的树形下拉，展开最多两级的任务计划树并选中一个节点，待办列表随即只显示该计划及其所有子孙计划下的到期待办。

## Requirements

### Requirement: Plan tree filter entry on pending page
待办任务页 SHALL 在工具栏「提前查询」开关右侧提供任务计划筛选树形下拉，点击即展开计划树。

#### Scenario: Entry visible
- **WHEN** 用户进入待办任务页
- **THEN** 工具栏「提前查询」开关右侧显示一个树形下拉控件，未筛选时展示占位文案「按任务计划筛选」

#### Scenario: Tree depth limited to two levels
- **WHEN** 用户点击筛选框并展开计划树
- **THEN** 每个根计划下最多展示其直接子计划（两层），更深的层级不展示；无子计划的根计划作为叶子节点展示

#### Scenario: Collapsed by default
- **WHEN** 用户首次点击筛选框
- **THEN** 计划树默认折叠，需点击展开箭头才展开层级

#### Scenario: Root plan selectable
- **WHEN** 用户点击第一层（根计划）节点文字
- **THEN** 该根计划被选中并回填到筛选框，面板关闭；不要求先展开子层

#### Scenario: Tree source
- **WHEN** 组件挂载
- **THEN** 前端调用 `GET /api/taskPlans/tree` 获取计划树并裁剪为两层；接口失败时仅记录日志，待办列表照常展示（不做筛选）

### Requirement: Filter semantics by plan subtree
选中计划节点后，待办列表 SHALL 只包含该计划及其所有子孙计划下的任务，且过滤在服务端生效。

#### Scenario: Filter by root plan
- **WHEN** 用户选中一个根计划
- **THEN** 请求 `GET /api/tasks/pending?planId=<根计划ID>`，响应 items 仅含该计划及其全部子孙计划下的待办，total 与筛选结果一致

#### Scenario: Filter by child plan
- **WHEN** 用户选中一个子计划
- **THEN** 列表仅含该子计划下的待办，不含其父计划与兄弟计划的任务

#### Scenario: Combine with early flag
- **WHEN** 同时开启「提前查询」并选中计划
- **THEN** 两个条件同时生效（未到点任务 + 指定计划子树）

#### Scenario: Pagination resets on filter change
- **WHEN** 用户切换或清除筛选条件
- **THEN** 页码重置为第 1 页后重新拉取列表

#### Scenario: Filter survives refresh triggers
- **WHEN** 筛选生效期间发生 30s 轮询，或用户完成 / 取消 / 延期 / 评分某条待办触发全局刷新
- **THEN** 列表仍在同一筛选条件下刷新，筛选条件不被重置

### Requirement: Clear filter
系统 SHALL 支持一键清除筛选，清除后恢复全量待办。

#### Scenario: Clear via control
- **WHEN** 用户在筛选中点击清空按钮
- **THEN** 筛选条件被清除，请求不再携带 planId，列表恢复全量待办并回到第 1 页

#### Scenario: Empty state wording
- **WHEN** 筛选生效且结果为空
- **THEN** 空状态文案为「该计划下暂无待办」；未筛选时仍为「暂无待办」
