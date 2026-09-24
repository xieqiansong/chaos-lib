## Context

- 前端 `chaos-ui/src/components/PendingTasks.vue` 是待办列表唯一实现（`views/PendingTask.vue` 只是薄壳），工具栏现有：`el-switch`（提前查询）+ 右侧「已选 N 项」/「批量延期」按钮（`.toolbar-actions`，`margin-left: auto`）
- 数据源：`GET /api/tasks/pending?early=&page=&size=`，服务端分页，响应 `{ items, total, page, size }`（见 `internal/pagination`）
- 计划树数据源：`GET /api/taskPlans/tree`（`taskplan.GetTaskPlanTree`），返回 `[{ID, ParentID, Name, PlanType, Status, IsSuspended, Children:[...]}]`，字段名为 Go 导出名（无自定义 json tag）
- 后端已有 `collectPlanWithDescendants(rootID) ([]int, error)`（`internal/taskplan/plan.go`），递归收集自身 + 全部子孙计划 ID，被 suspend / priority 级联复用
- 前端 Element Plus 由 `unplugin-vue-components` 自动导入，`el-tree-select` 无需手动 import
- 跨组件刷新走 `utils/pendingTasksStore.ts` 的 `pendingTasksVersion` 信号；组件内 30s 轮询

## Goals / Non-Goals

**Goals:**

- 待办列表可按任务计划收敛，选中父计划即覆盖其子树
- 树形下拉最多两级，避免计划树很深时下拉过长
- 筛选与服务端分页、`total`、`early` 开关、30s 轮询、刷新信号全部协同工作
- 复用既有接口与计划 ID 收集逻辑，不新增路由、不改表结构

**Non-Goals:**

- 不做多选筛选（一次只筛一个计划节点；「不选」即全量）
- 不做前端本地过滤（列表是服务端分页，本地过滤会导致 total 与页数错乱）
- 不在筛选下拉里展示计划状态 / 待办数量等附加信息（保持纯树形选择）
- 不持久化筛选条件到 localStorage / URL query（离开页面后重置，符合轻量工具定位）
- 不改动 `taskPlans/tree` 接口的返回结构

## Decisions

### D1: 后端加 `planId` 参数，而不是前端本地过滤
列表是服务端分页（`pagination.Parse` + `Limit/Offset`），本地过滤只能过滤当前页，`total` 与分页器必然失真。因此在 `GetPendingTasks` 的 base 查询（count 与分页查询共用）上追加过滤条件，`total` 自动收敛。

### D2: 用 `collectPlanWithDescendants` 展开子树，而非只匹配单个 plan_id
「选一个计划」的直觉是看该计划整体的待办，任务常常挂在叶子子计划上而不是父计划。复用既有函数（与挂起 / 优先级级联同一套语义），无需新写递归。代价是对深层树有一次递归查询，个人工具数据量下可忽略。

### D3: 树只用两层，但筛选范围含全部后代
下拉展示裁剪到两层（根 + 直接子计划），符合「最多两个层级就够了」的诉求；筛选范围仍是所选节点的**全部后代**——若只筛两层，选中根计划时第三层及更深的待办会神秘消失，与用户预期相反。

裁剪在前端 `loadPlanTree()` 里做：`[{ID, Name, Children: (node.Children || []).map(c => ({ID: c.Name... }))}]`，子节点不再携带 `Children`，`el-tree-select` 自然渲染为叶子。

### D4: 控件选 `el-popover + el-tree` 而非 `el-tree-select`
`el-tree-select` 在该版本单选模式下父节点（第一层）的点击行为不可控——点父节点文字只会展开而不回填选中，导致「只能选第二层」。改用 `el-popover`（触发为只读 `el-input`）+ `el-tree` 完全自控：
- 计划树在 `toPlanTreeOptions()` 中映射为 `{value, label, children}`（`value` 取计划 ID、`label` 取计划名），直接匹配 `el-tree` 默认 props，无需自定义 `:props`
- `el-tree` 默认**折叠**（`el-tree` 默认不展开），用户点展开箭头才展开两层；`:expand-on-click-node="false"` 使得点节点文字只触发选中、不会误展开
- 任意节点（含第一层根计划）点击经 `@node-click="onPlanNodeClick"` 回填 `filterPlanId` / `filterPlanLabel` 并关闭面板；`:highlight-current` + `:current-node-key` 回显当前筛选项
- 只读 `el-input` 的 `clearable` + `@clear="clearPlanFilter"` 提供一键清除；整个交互无需自维护弹层定位 / 外部点击关闭（`el-popover` 已处理）

### D5: 筛选变更 = 回第 1 页 + 重拉；不清空勾选以外的任何状态
- `handlePlanFilterChange()`：`page = 1` 后 `loadPendingTasks()`，避免落在筛选后不存在的页码上（与 `handleEarlyModeChange` 同一套路）
- `loadPendingTasks()` 在参数串末尾按需追加 `&planId=`，`early` 与分页参数位置不变
- 30s 轮询 / `pendingTasksVersion` 走同一个 `loadPendingTasks()`，因此筛选条件天然被保留，无需额外处理
- 空状态文案：`filterPlanId` 非空时显示「该计划下暂无待办」，否则维持「暂无待办」

## Risks / Trade-offs

- 计划树接口返回全量计划（含已归档 / 已完成 / 已挂起）：下拉里可能选到「已挂起」计划，此时后端 `is_suspended = false` 条件生效，列表必为空 —— 属于既有语义（挂起计划本就不产生待办），空状态文案可解释
- 两层裁剪后，若某根计划的待办全在第三层以下，用户在根节点上看到的结果是「包含全部后代」，与下拉里看到的层级不完全对应 → 已在 D3 明确取舍（宁可多包含，不可漏掉）
- 计划树在 `onMounted` 只拉一次：若在别处新建计划后回到待办页，下拉可能滞后 → 组件重新挂载（路由切换）即恢复；不做额外轮询以免增加请求
- 纯读接口扩展，无数据风险；回滚只需去掉 `planId` 分支与前端控件

## Migration Plan

- 后端：改 `GetPendingTasks` 一个函数即可，无 DB / 路由变更，重新 `go build` 出二进制
- 前端：`chaos-ui` 重新 `pnpm build`，`dist/` 拷到 exe 同目录 `ui/`（`scripts/chaos.deploy.ps1`），无需重编后端
- 回滚：移除 `planId` 分支与工具栏控件，前端旧版本与新后端完全兼容（`planId` 缺省即全量）

## Open Questions

- 是否需要把筛选条件同步到 URL / localStorage（刷新后保留）？当前实现为「离开页面即重置」，如需保留再单开变更
- 是否需要在下拉节点上展示该计划下的待办数量角标？当前不做（会引入额外统计请求）
