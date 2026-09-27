## Context

- 后端 `chaos-go/internal/taskplan/handler.go`：`PostponeTask`（单条）规则为「仅 `TaskStatusActive` 且 plan 类型为 `todo`/`interval` 的任务可延期，平移 `started_at` 与 `deadline` 各 `days * 24h`」，返回 `{message, days}`；`cron` 类型直接拒绝
- 路由 `chaos-go/routes/routes.go` 的 `tasks` 组：`GET /pending`、`PATCH /:id/complete`、`PATCH /:id/cancel`、`PATCH /:id/postpone`；本次新增 `POST /batch-postpone`
- 前端 `chaos-ui/src/components/PendingTasks.vue`：消费 `GET /api/tasks/pending`，已有单条「延期」对话框（`postponePresets=[1,3,7]` + `el-input-number`），复选框列与批量按钮是新增
- 前端请求统一走 `chaos-ui/src/utils/api.ts` 的 `sendMessage`（原生 fetch，无 axios）
- 勾选状态、批量提交、弹窗复用均在 `PendingTasks.vue` 内闭环，不引入新 store（全局刷新仍走既有 `pendingTasksStore.pendingTasksVersion`）

## Goals / Non-Goals

**Goals:**

- 列表可勾选（`todo`/`interval` 可勾，`cron` 禁勾），勾选后「批量延期」可用
- 批量延期复用单条弹窗：相同天数、相同预设、相同时间平移语义
- 批量接口具备「部分成功」能力：单个不可延期（已完成 / 非 todo·interval / 无时间字段）的任务被跳过，其余照常延期，返回每条结果
- 单条延期行为完全不变（重构内聚，对外语义一致）

**Non-Goals:**

- 不做「按各自相对时间分别延期」（如每条延到各自的下个整点）——本次统一按相同天数平移
- 不支持跨计划的差异化天数（一个弹窗一个天数）
- 不新增 / 修改数据库字段，不追加 `chaos_postgres_update.sql`
- 不对 `cron` 任务开放延期（与单条规则一致，仅禁用勾选并跳过）

## Decisions

### D1: 抽离 `computePostponeUpdates(task, days)` 公共逻辑
单条 `PostponeTask` 与批量 `BatchPostponeTasks` 的约束（状态 / plan 类型 / 字段平移）完全相同。把「能否延期 + 计算更新字段」抽成纯函数返回 `(updates, ok, reason)`，两处共用，避免漂移。

- 为什么抽函数而不是在批量里重写一遍：约束一旦变更（如未来放开某类型），只改一处
- 返回 `reason`：批量场景需要把「为什么跳过」回传前端；单条场景直接当作错误信息返回

### D2: 批量接口返回结构化 `{postponed, skipped, details, message}`
逐条 `db.Model(&task).Updates(updates)`，每条结果入 `details`：`{id, status:"postponed"|"skipped", reason?}`。

- 为什么不全有成功才成功：用户勾选一批，其中个别已完成不应让整批失败；部分成功对用户更友好
- 为什么不包事务：每条独立、无相互依赖；单条失败即整体 500 并终止（罕见，如 DB 写入错误），已延期的不回滚——个人工具可接受

### D3: 前端勾选列用 `reserve-selection` + `row-key="ID"`
列表每 30s 轮询刷新（`loadPendingTasks`），`reserve-selection` 按 `ID` 保留勾选，刷新后用户选择不丢；`:selectable="isPostponable"` 让 `cron` 行不可勾，从源头避免误选。

- 提交成功后手动清空 `selectedTasks` 并 `refreshPendingTasks()`；若任务延期后仍在列表（仍 pending）会因 `reserve-selection` 保持勾选，下一次点击仍会带上——属预期，用户可手动取消勾选

### D4: 复用同一延期弹窗，用 `postponeDialogTitle` 区分单条 / 批量
提交逻辑 `submitPostponeDialog` 内判断：`postponeTargetTask` 非空 → 单条；否则 → 批量取 `selectedTasks.map(ID)`。避免再建一个弹窗与一套天数 UI。

- 为什么复用：预设按钮、自定义天数、校验（`days>0`）完全一致，复制一套只增维护成本
- 提示分级：`skipped>0` 用 `ElMessage.warning`（部分跳过），否则 `success`

## Risks / Trade-offs

- 批量提交后个别任务仍在列表且保持勾选（`reserve-selection`），可能造成「再次延期」误包含——已用「已选 N 项」计数提示，用户可见；非阻塞
- 批量接口无事务回滚：极端 DB 错误会导致已延期条目不回滚。个人单机工具、数据量小、发生概率低；如需强一致可后续加事务
- 前端 `el-table` 勾选在大数据量下依赖 `row-key` 正确性，`ID` 为唯一主键，安全
- 无数据库变更，回滚风险为零（删 handler + 路由 + 组件改动即可）

## Migration Plan

- 无数据库迁移（`chaos_postgres_update.sql` 不动）
- 部署：`chaos-go` 重新 `go build`；`chaos-ui` 重新 `pnpm build` 后 `dist/` 拷入后端静态资源目录（`scripts/chaos.deploy.ps1`），无需改 `.env`
- 回滚：删除 `BatchPostponeTasks`、路由一行、组件勾选列 / 批量按钮与 `api.ts` 两个函数即可

## Open Questions

- 是否需要在批量延期后自动清除仍在列表的勾选（而非保留）？当前保留以方便「再延一次」，若用户更想要「提交即清空」可调整 `reserve-selection` 行为。
