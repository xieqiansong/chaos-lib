## 1. 后端批量延期接口

- [x] 1.1 `chaos-go/internal/taskplan/handler.go`：抽离 `computePostponeUpdates(task *Task, days int) (map[string]interface{}, bool, string)`，承载状态 / plan 类型校验与时间平移计算
- [x] 1.2 `PostponeTask` 改为调用 `computePostponeUpdates`，语义与返回保持不变
- [x] 1.3 新增 `BatchPostponeTasks`：`POST` 入参 `{ ids:[]int, days:int }`，校验 `days>0` 与 `ids` 非空，逐条应用 `computePostponeUpdates` 并 `Updates`，返回 `{postponed, skipped, details:[{id,status,reason?}], message}`
- [x] 1.4 `chaos-go/routes/routes.go`：`tasks` 组注册 `POST /batch-postpone`

## 2. 前端 API 与待办列表勾选 + 批量延期

- [x] 2.1 `chaos-ui/src/utils/api.ts`：新增 `postponeTask(id, days)` 与 `batchPostponeTasks(ids, days)`
- [x] 2.2 `chaos-ui/src/components/PendingTasks.vue`：`el-table` 增加 `row-key="ID"` 与 `@selection-change`，新增 `type="selection"` 勾选列（`:selectable="isPostponable" reserve-selection`）
- [x] 2.3 `PendingTasks.vue`：新增 `selectedTasks` 状态、`handleSelectionChange`、`isPostponable(row)`（`todo`/`interval` 可勾）、`batchPostpone()` 入口
- [x] 2.4 `PendingTasks.vue`：工具栏新增「批量延期」按钮（`:disabled="selectedTasks.length===0"`）与「已选 N 项」计数；新增 `.toolbar-actions` / `.selected-count` 样式
- [x] 2.5 `PendingTasks.vue`：复用延期弹窗，新增 `postponeDialogTitle` 计算属性（单条显示任务名 / 批量显示「批量延期（已选 N 个）」）；`submitPostponeDialog` 区分单条与批量分支，批量成功后清空 `selectedTasks` 并 `refreshPendingTasks()`；`skipped>0` 用 warning 提示

## 3. 规格与构建

- [x] 3.1 `openspec/specs/task-plan/spec.md`：API Endpoints 增加 `POST /api/tasks/batch-postpone`，并在「Postpone only for todo/interval」需求下补充批量场景
- [x] 3.2 `openspec/changes/batch-postpone-tasks/` 补充 `specs/task-plan/spec.md` 增量规格（四件套齐备）
- [x] 3.3 后端 `go build ./...` 通过（taskplan / routes 包编译无误）
- [ ] 3.4 后端补充 `BatchPostponeTasks` 单元测试（含跳过规则）——用户决定是否添加，未强写
- [ ] 3.5 前端 `pnpm build` 通过（用户本地构建验收）
- [ ] 3.6 前端手动验证清单（见下，用户人工验收）

## 4. 前端手动验证清单（用户人工验收）

- [ ] 4.1 待办列表首列出现勾选框；`cron` 周期任务行勾选框置灰不可勾，`todo`/`interval` 可勾
- [ ] 4.2 未勾选时「批量延期」按钮禁用；勾选至少一项后按钮可用并显示「已选 N 项」
- [ ] 4.3 点击「批量延期」弹窗标题为「批量延期（已选 N 个）」，预设 1/3/7 天与自定义天数同单条一致
- [ ] 4.4 选多条 `todo`/`interval` 提交相同天数，全部开始时间 / 截止时间（若有）按天数统一顺延，列表刷新后体现
- [ ] 4.5 勾选中混入已完成 / 不可延期任务时，批量结果提示「已延期 X 个任务，跳过 Y 个」（warning），可延期的部分仍成功
- [ ] 4.6 单条「延期」按钮行为不变，提示「已延期 N 天」（success）
- [ ] 4.7 列表 30s 轮询刷新后，已勾选项因 `reserve-selection` 保持勾选，「已选 N 项」计数不变
- [ ] 4.8 批量成功后勾选清空、列表即时刷新（经 `pendingTasksVersion` 信号），页头待办角标（若有）同步减少
