## MODIFIED Requirements

### Requirement: Pending task query
系统 SHALL 提供联表待办查询，排除已挂起计划并标记逾期；并 SHALL 支持按任务计划（含其所有子孙计划）收敛结果集。

#### Scenario: Pending excludes suspended plans
- **WHEN** 请求 `/tasks/pending`
- **THEN** 不含 isSuspended=true 的计划下的 Task

#### Scenario: Early flag
- **WHEN** 带 `?early=1`
- **THEN** 包含未到点（startedAt 在未来）的 Task

#### Scenario: Overdue marking
- **WHEN** Task 有 deadline 且已过
- **THEN** 该条 isOverdue=true

#### Scenario: Filter by plan subtree
- **WHEN** 请求 `/tasks/pending?planId=<计划ID>`（正整数）
- **THEN** 仅返回 plan_id 属于该计划及其所有子孙计划的 Task，total 与分页结果一致

#### Scenario: Filter combines with other params
- **WHEN** `planId` 与 `early=1`、`page`、`size` 同时传入
- **THEN** 各条件同时生效；缺省 `planId` 时行为与既有一致（全量待办）

#### Scenario: Invalid planId rejected
- **WHEN** `planId` 非正整数或无法解析
- **THEN** 返回 400「无效的计划ID」，不执行查询

## API Endpoints (changed)

```
GET    /api/tasks/pending                   待办（?early=1&planId=<计划ID>）
```
