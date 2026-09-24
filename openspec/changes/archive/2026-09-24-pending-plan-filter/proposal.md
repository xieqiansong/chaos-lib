## Why

「待办任务」页目前只能整体查看所有到期待办：工具栏只有「提前查询」开关与「批量延期」，没有任何维度可以把列表收敛到某个任务计划。实际使用中待办往往横跨多个计划（大计划下还嵌套子计划），想只看「某个计划及其子计划」的待办，只能靠肉眼在列表里逐行翻找，计划一多就不可用。

## What Changes

- 待办任务页工具栏（`PendingTasks.vue`，位于「提前查询」开关右侧）新增**任务计划筛选**树形下拉：点击即展开任务计划树，**最多展示两个层级**（根计划 + 其直接子计划），选中任一节点即按该计划筛选
- 筛选语义：选中计划后，待办列表只保留该计划**及其所有子孙计划**下的任务；过滤在服务端完成，`total` 与实际行数同步收敛，分页正常
- 支持清除筛选（下拉自带清空按钮）：清除后恢复全量待办
- 切换筛选条件时回到第 1 页；筛选条件在 30s 轮询、`pendingTasksVersion` 刷新信号（完成 / 取消 / 延期 / 评分）后保持不变
- 后端 `GET /api/tasks/pending` 新增**可选** `planId` 参数：按该计划及所有子孙计划的 ID 集合过滤；非法 `planId`（非正整数）返回 400
- 计划树复用既有 `GET /api/taskPlans/tree`，不新增接口、不新增数据库字段

## Capabilities

### New Capabilities
- `pending-plan-filter`: 待办任务按任务计划树筛选的能力，定义树形下拉的位置与展示深度、筛选语义（含子孙计划）、清除行为、分页联动与前后端契约

### Modified Capabilities
- `task-plan` spec：`Pending task query` 需求新增「按 planId 筛选」场景，API Endpoints 标注 `?planId=`

## Impact

- 后端 `chaos-go/internal/taskplan/handler.go`：`GetPendingTasks` 读取 `planId`，经既有 `collectPlanWithDescendants` 收集计划 ID 集合，向 base 查询追加 `tasks.plan_id IN ?`；无新增模型 / 表结构变更
- 前端 `chaos-ui/src/components/PendingTasks.vue`：工具栏新增 `el-tree-select` 树形筛选控件与 `planTree` / `filterPlanId` 状态、`loadPlanTree()`、`handlePlanFilterChange()`；`loadPendingTasks()` 组装 `planId` 参数
- `chaos-ui/src/utils/api.ts`：无需新增（沿用 `sendMessage`）；`taskPlans/tree`、`tasks/pending` 路由均已注册
- **无数据库模型变更**：不新增 / 修改 GORM 字段，`chaos_postgres_update.sql` 无需追加
- 前端筛选控件内的计划名与既有页面一致（直接取树接口的 `Name`），不做额外脱敏处理
