## 1. 后端按计划筛选待办

- [x] 1.1 `chaos-go/internal/taskplan/handler.go`：`GetPendingTasks` 读取可选查询参数 `planId`，非空时用 `strconv.Atoi` 校验（须为正整数，否则 400「无效的计划ID」）
- [x] 1.2 `GetPendingTasks`：调用既有 `collectPlanWithDescendants(planID)` 收集该计划及所有子孙 ID，向 base 追加 `Where("tasks.plan_id IN ?", ids)`（在 count 与分页查询之前，保证 total 同步收敛）
- [x] 1.3 确认 `planId` 与 `early`、分页参数可叠加使用，筛选后的 `Order` / `Scopes(q.Scope)` 行为不变

## 2. 前端待办页树形筛选

- [x] 2.1 `chaos-ui/src/components/PendingTasks.vue`：新增 `PlanTreeNode` 接口（`value` / `label` / `children`）与 `planTree` / `filterPlanId` 状态；`loadPlanTree()` 调 `sendMessage('taskPlans/tree', 'GET')`，把返回树裁剪为**两层**（根 + 直接子计划，更深层级丢弃并在选中根时由后端包含）
- [x] 2.2 `PendingTasks.vue`：工具栏「提前查询」开关右侧新增 `el-popover`（触发为只读 `el-input`，占位「按任务计划筛选」）包裹 `el-tree`（`node-key="value"`、`:expand-on-click-node="false"`、`:highlight-current="true"`、`:current-node-key` 回显）；`el-tree` 数据裁剪为两层，`@node-click="onPlanNodeClick"` 选中含第一层根计划在内的任意节点并关闭面板；只读 `el-input` 不显示自带 `clearable` 图标，故在 `#suffix` 自绘 `CircleClose` 清除按钮（有筛选值时显示，`@click.stop.prevent="clearPlanFilter"`）
- [x] 2.3 `PendingTasks.vue`：`handlePlanFilterChange()` 将 `page` 重置为 1 并 `loadPendingTasks()`；`loadPendingTasks()` 在 `filterPlanId` 非空时向 URL 追加 `&planId=${filterPlanId}`
- [x] 2.4 `PendingTasks.vue`：`onMounted` 调用 `loadPlanTree()`（失败仅 console 记录，不阻塞待办列表）；筛选态下空列表文案区分「该计划下暂无待办」

## 3. 规格与构建

- [x] 3.1 `openspec/specs/task-plan/spec.md`：`Pending task query` 需求补充 `planId` 场景，API Endpoints 的 `GET /api/tasks/pending` 标注 `?early=1&planId=`
- [x] 3.2 `openspec/changes/pending-plan-filter/` 补齐 `specs/pending-plan-filter/spec.md` 与 `specs/task-plan/spec.md` 增量（四件套齐备）
- [x] 3.3 后端 `go build ./...` 通过
- [x] 3.4 后端 `go test ./...` 已实际执行：`taskplan` / `routes` 等本次涉及包无失败；仅 `cmd/tcp_over_websockets`（vet: redundant newline）与 `tools`（vet: slog.Error 参数）两个**与本次无关的预存失败**，未在本次改动范围内修复
- [ ] 3.5 后端补充 `GetPendingTasks` 的 `planId` 单元测试（含非法入参 / 含子计划）——用户决定是否添加，未强写
- [x] 3.6 前端 `pnpm build` 通过
- [ ] 3.7 前端手动验证清单（见下，用户人工验收）

## 4. 前端手动验证清单（用户人工验收）

- [ ] 4.1 待办任务页工具栏「提前查询」开关右侧出现筛选下拉，占位文案为「按任务计划筛选」
- [ ] 4.2 点击筛选框默认不展开计划树；点展开箭头后树最多只有两个层级（根计划与其直接子计划），层级更深的计划不显示
- [ ] 4.3 选中一个**根计划**（第一层）：列表只剩该计划及其所有子孙计划下的待办，`total` 与行数一致；分页可正常翻页
- [ ] 4.4 选中一个**子计划**（第二层）：列表只剩该子计划下的待办（不含父计划或其他兄弟计划的任务）
- [ ] 4.5 已选中计划时筛选框右侧出现清除按钮（未筛选时不显示）：点击后筛选条件清除，列表恢复全量待办，页码回到 1，且不会展开计划树面板
- [ ] 4.6 筛选状态下点「完成 / 取消 / 延期」或等 30s 轮询：筛选条件保持，列表仍在筛选范围内刷新
- [ ] 4.7 勾选某计划下若干可延期任务后「批量延期」：成功后筛选条件与页码保持，列表即时刷新
- [ ] 4.8 选中一个没有任何到期待办的计划：显示空状态且文案提示为「该计划下暂无待办」
- [ ] 4.9 打开「提前查询」并叠加筛选：两个条件同时生效（未到点 + 指定计划）
