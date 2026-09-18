## Purpose

待办任务支持「勾选 + 批量延期」：对同一组 active 且为 `todo`/`interval` 类型的待办，按相同天数统一顺延 `started_at` 与 `deadline`，并具备部分成功（跳过不可延期项）能力。

## Requirements

### Requirement: Batch postpone pending tasks
系统 SHALL 提供批量延期接口，对同一组任务应用相同天数延期，逐条跳过不可延期的任务并返回每条结果。

#### Scenario: Batch postpone postponable tasks
- **WHEN** `POST /api/tasks/batch-postpone` 传入若干 `todo`/`interval` 类型 active 任务的 `ids` 与正整数 `days`
- **THEN** 这些任务的 `started_at` 与 `deadline`（若存在）各平移 `days` 天；响应 `postponed` 等于成功数、`skipped=0`

#### Scenario: Batch postpone skips unsupported tasks
- **WHEN** 传入的 `ids` 含已完成 / 已取消、`cron` 类型或无可延期时间的任务
- **THEN** 这些任务被跳过并计入 `skipped`，`details` 中给出各自 `reason`；其余可延期任务仍成功，`postponed` 为实际成功数

#### Scenario: Invalid request rejected
- **WHEN** `days<=0` 或 `ids` 为空
- **THEN** 返回 400 与相应错误提示，不执行任何延期

### Requirement: Postpone only for todo/interval (batch consistent)
批量延期 MUST 复用单条延期的约束：仅 `TaskStatusActive` 且 plan 类型为 `todo`/`interval` 的任务可延期。

#### Scenario: Postpone only for todo/interval (batch)
- **WHEN** 对 `cron` 类型任务调用批量延期（无论单条还是批量）
- **THEN** 该任务被跳过（批量）或被拒绝（单条），理由为「仅待办和间隔类型任务支持延期」
