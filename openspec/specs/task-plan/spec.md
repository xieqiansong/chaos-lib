# Spec: Task Plan (任务计划)

## Purpose

管理可复习、可编排的个人任务。一个任务计划（TaskPlan）按类型产生若干执行实例（Task），支持树状组织、状态流转、FSRS 间隔复习与统计。

## Requirements

### Requirement: TaskPlan tree structure
系统 SHALL 以树状组织任务计划，支持创建、查询、更新与删除。

#### Scenario: Create root and child plan
- **WHEN** 创建任务计划可不传 parentId（根）或传已有计划 id（子）
- **THEN** 返回新建计划 id 及其字段

#### Scenario: List as tree with search
- **WHEN** 请求 `/taskPlans/tree` 且带 `?search=关键字`
- **THEN** 返回匹配节点及其所有祖先构成的树

#### Scenario: Partial update
- **WHEN** PATCH 提交部分字段
- **THEN** 仅更新传入字段，其余保持不变

#### Scenario: Delete with active tasks blocked
- **WHEN** 删除含子计划或活跃 Task 的计划且未带 `?cascade=true`
- **THEN** 拒绝并返回 childCount / activeTaskCount 提示

#### Scenario: Cascade delete
- **WHEN** 删除带 `?cascade=true`
- **THEN** 计划及其子孙计划与关联 Task 全部软删除

### Requirement: Plan lifecycle states
系统 SHALL 支持计划状态流转 created → started → completed → archived，并支持挂起/恢复与优先级级联。

#### Scenario: Start a plan
- **WHEN** 对 created/started 状态的计划调用 start
- **THEN** 状态变 started，若无活跃 Task 则生成一条

#### Scenario: Complete a plan
- **WHEN** 对 started 状态的计划调用 complete
- **THEN** 其活跃 Task 标记 done，计划变 completed

#### Scenario: Archive requires completion
- **WHEN** 对非 completed 状态调用 archive
- **THEN** 拒绝

#### Scenario: Suspend cascades to descendants
- **WHEN** 挂起某计划
- **THEN** 该计划及其所有子孙 isSuspended 置 true

#### Scenario: Priority cascades to descendants
- **WHEN** 设置优先级
- **THEN** 该计划及其所有子孙 priority 更新

### Requirement: Task generation by plan type
系统 SHALL 按 planType 自动产生并管理 Task 执行实例。

#### Scenario: Todo creates first task on creation
- **WHEN** 创建 todo 类型且提供 startedAt
- **THEN** 立即生成第一条 Task

#### Scenario: Interval requires rating on complete
- **WHEN** 完成 interval 类型 Task 未传 rating
- **THEN** 拒绝，提示有效 rating 取值

#### Scenario: Complete generates next task
- **WHEN** 完成 cron / interval 类型 Task
- **THEN** 按类型生成下一条 Task（interval 同时更新 FSRS 卡片参数）

#### Scenario: Cancel only for recurring types
- **WHEN** 取消非 cron/interval 类型 Task
- **THEN** 拒绝

#### Scenario: Postpone only for todo/interval
- **WHEN** 对 cron 类型 Task 调用 postpone
- **THEN** 拒绝；对 todo/interval 按天数平移 startedAt 与 deadline

### Requirement: Pending task query
系统 SHALL 提供联表待办查询，排除已挂起计划并标记逾期。

#### Scenario: Pending excludes suspended plans
- **WHEN** 请求 `/tasks/pending`
- **THEN** 不含 isSuspended=true 的计划下的 Task

#### Scenario: Early flag
- **WHEN** 带 `?early=1`
- **THEN** 包含未到点（startedAt 在未来）的 Task

#### Scenario: Overdue marking
- **WHEN** Task 有 deadline 且已过
- **THEN** 该条 isOverdue=true

### Requirement: Statistics
系统 SHALL 提供每日完成数与活跃任务数的统计接口。

#### Scenario: Daily completion stats
- **WHEN** 请求 `/tasks/dailyStats`
- **THEN** 返回近 30 天每日完成数

#### Scenario: Active stats
- **WHEN** 请求 `/tasks/activeStats`
- **THEN** 返回近 7 天每日活跃 Task 数

## API Endpoints (current)

```
POST   /api/taskPlans/                      创建
GET    /api/taskPlans/                      列表
GET    /api/taskPlans/tree                  树形（?search=）
GET    /api/taskPlans/:id                   详情
PATCH  /api/taskPlans/:id                   更新
PATCH  /api/taskPlans/:id/start
PATCH  /api/taskPlans/:id/complete
PATCH  /api/taskPlans/:id/archive
PATCH  /api/taskPlans/:id/suspend
PATCH  /api/taskPlans/:id/resume
PATCH  /api/taskPlans/:id/priority
DELETE /api/taskPlans/:id                   (?cascade=true)
GET    /api/taskPlans/:id/tasks             某计划下任务

GET    /api/tasks/pending                   待办（?early=1）
GET    /api/tasks/dailyStats
GET    /api/tasks/activeStats
PATCH  /api/tasks/:id/complete
PATCH  /api/tasks/:id/cancel
PATCH  /api/tasks/:id/postpone
```

## Out of Scope
- 前端 UI 实现（chaos-ui 自行消费上述 API）
- FSRS 算法具体数学公式（见 internal/taskplan/fsrs.go）
- 后台扫计划生成 Task 的调度逻辑（见 internal/taskplan/scheduler.go）
