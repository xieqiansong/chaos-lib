## Why

待办任务页（待办列表 `PendingTasks.vue`）目前只支持**单条**「延期」：每行一个「延期」按钮，弹出对话框选天数后只延期当前这一条。日常场景里经常需要把一批同类待办（例如多个 todo 类型的周任务、或一批到期的 interval 复习）统一往后顺延相同天数，逐条点开效率很低。缺少「勾选 + 批量延期」能力。

## What Changes

- 待办列表 `el-table` 新增**勾选列**（复选框），仅 `todo` / `interval` 类型可勾选（`cron` 周期任务不可延期，禁用勾选）
- 工具栏新增「批量延期」按钮（右侧，与「提前查询」开关同栏），仅在已勾选至少一项时可用；显示「已选 N 项」计数
- 点击「批量延期」复用既有延期对话框（预设 1/3/7 天 + 自定义天数），提交时按**相同天数**批量延期所有勾选项
- 后端新增 `POST /api/tasks/batch-postpone`，入参 `{ ids: number[], days: number }`，逐条应用单条延期的同一规则（仅 active + todo/interval，平移 `started_at` / `deadline`），对不可延期的任务**跳过**而非整体失败
- 返回结构化结果：`{ postponed, skipped, details:[{id,status,reason?}], message }`，前端按 skipped 数量用 warning / success 提示，部分成功可感知
- 单条「延期」按钮与逻辑保持不变（重构为复用公共 `computePostponeUpdates`，语义无变化）

## Capabilities

### New Capabilities
- `batch-postpone-tasks`: 待办列表勾选 + 批量延期能力，定义勾选范围、批量接口入参 / 出参、跳过规则与前端交互

### Modified Capabilities
- `task-plan` spec：新增 `POST /api/tasks/batch-postpone` 端点与「批量延期」场景（复用单条延期的类型 / 状态约束）

## Impact

- 后端 `chaos-go/internal/taskplan/handler.go`：新增 `BatchPostponeTasks`，抽离 `computePostponeUpdates` 供 `PostponeTask` 与批量复用（纯逻辑内聚，无 DB 模型变更）
- 后端 `chaos-go/routes/routes.go`：在 `tasks` 组注册 `POST /batch-postpone`
- 前端 `chaos-ui/src/utils/api.ts`：新增 `postponeTask` / `batchPostponeTasks`
- 前端 `chaos-ui/src/components/PendingTasks.vue`：新增勾选列、`handleSelectionChange` / `isPostponable`、`batchPostpone` 入口、复用 `postponeDialogTitle` 与提交逻辑；新增工具栏样式
- **无数据库模型变更**：不新增 / 修改任何 GORM 模型字段，`chaos_postgres_update.sql` 无需追加
- 与单条延期一致性：批量延期的天数语义、类型约束、时间平移逻辑与 `PostponeTask` 完全一致
