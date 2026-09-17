## 1. 路由与菜单拆分

- [x] 1.1 `chaos-ui/src/router/index.ts`：`/task` 拆为 `/taskPlan`（title「任务计划」, icon `Tickets`）与 `/pendingTask`（title「待办任务」, icon `Clock`），均指向新视图
- [x] 1.2 `chaos-ui/src/router/index.ts`：保留 `{path: '/task', redirect: '/taskPlan', meta: {hidden: true}}` 兼容旧地址
- [x] 1.3 `chaos-ui/src/App.vue`：`CMD_ALIAS` 由 `task` 改为 `taskPlan: 'task'`，新增 `pendingTask: 'todo'`

## 2. 页面拆分

- [x] 2.1 新建 `chaos-ui/src/views/TaskPlan.vue`：迁移原「任务计划」全部逻辑（树加载 / 懒加载展开恢复 / 搜索展开 / 新建 / 子任务 / 编辑 / 开启 / 完成 / 归档 / 挂起 / 恢复 / 优先级 / 复习评分对话框）
- [x] 2.2 `TaskPlan.vue` 移除 `activeTab` / `pendingRef` / `loading`（骨架屏）与模板内的 tabs、待办分支；`refreshAll()` 只保留 `refreshPendingTasks()` 全局广播，`refreshAllPlans()` 直接 `fetchAllPlans()`
- [x] 2.3 新建 `chaos-ui/src/views/PendingTask.vue`：页头「待办任务」+ `<PendingTasks/>`，声明可选 `searchText` props 以免注入属性落到 DOM
- [x] 2.4 删除 `chaos-ui/src/views/Task.vue`
- [x] 2.5 `chaos-ui/src/App.vue`：移除侧边栏「待办任务」折叠分隔标题（`sidebar-divider`）与其下方常驻待办小列表（`sidebar-todo`），以及侧边栏 header 的「N 待办」徽标；待办任务只经菜单打开
- [x] 2.6 `chaos-ui/src/components/PendingTasks.vue`：移除 `view` prop 与 `sidebar` 列表分支、移除 `taskCount` / `refresh` 事件与对应 `emit` 调用，组件仅保留表格视图；清理相关死 CSS

## 3. 文档

- [x] 3.1 `README.md` 前端页面路由表：`/task 任务管理` 拆为 `/taskPlan 任务计划` 与 `/pendingTask 待办任务`

## 4. 构建与验收

- [x] 4.1 `chaos-ui` 类型检查 / 构建通过（`pnpm build`）
- [ ] 4.2 前端手动验证清单（见下）

## 5. 前端手动验证清单（用户人工验收）

- [ ] 5.1 左侧菜单不再出现「任务管理」，而是并列出现「任务计划」与「待办任务」两项，顺序与图标正常
- [ ] 5.2 点击「任务计划」进入独立页面，无 tab 切换栏，页头标题为「任务计划」，面包屑为「首页 / 任务计划」
- [ ] 5.3 任务计划页原有能力可用：新建根任务、添加子任务、展开 / 折叠子树、修改、开启、完成、归档、挂起 / 恢复、设置优先级、进度列显示、「更多」下拉菜单
- [ ] 5.4 顶部搜索框输入关键字后，任务计划树按关键字过滤并自动展开少量匹配命中
- [ ] 5.5 点击「待办任务」进入独立页面，无 tab 切换栏，页头标题为「待办任务」，面包屑为「首页 / 待办任务」
- [ ] 5.6 待办任务页能力可用：提前查询开关、完成、取消（周期任务）、延期（待办 / 间隔任务）、跳转、预览原文、复习（间隔任务）、逾期标记、复习次数列
- [ ] 5.7 在待办任务页完成 / 延期一条任务后，列表立即刷新（无需侧边栏同步）；重新进入待办页数据最新
- [ ] 5.8 在任务计划页挂起 / 恢复 / 完成一个计划后，切到待办任务页看到对应变化
- [ ] 5.9 直接访问旧地址 `#/task` 自动跳转到 `#/taskPlan`，不出现空白页
- [ ] 5.10 `Ctrl/Cmd+K` 命令面板中两个入口分别显示为「任务计划（task）」「待办任务（todo）」，且能正确跳转
- [ ] 5.11 刷新页面后停留在当前页面，菜单高亮正确
