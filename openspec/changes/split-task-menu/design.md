## Context

- 前端 chaos-ui：Vue 3 + TS + Vite + Element Plus，hash 路由
- 左侧菜单完全由 `chaos-ui/src/router/index.ts` 的 `appRoutes` 自动生成：`meta.title` / `meta.icon` 即菜单项，`meta.hidden` 不进菜单（见 `buildMenu`）
- 现状：只有一条 `/task` 路由指向 `views/Task.vue`，页内用 `el-tabs` 在「任务计划（all）」与「待办任务（pending）」间切换
- `Task.vue` 同时持有两类状态：任务树（`allPlans` / `childrenMap` / 懒加载展开恢复）与待办（`activeTab` / `pendingRef`），并让两者互相触发刷新
- 待办列表本身已是独立组件 `components/PendingTasks.vue`（仅保留表格视图），内部自带 30s 轮询、提前查询开关、完成 / 取消 / 延期 / 预览 / 复习
- 跨组件刷新走轻量 store：`utils/pendingTasksStore.ts`（`pendingTasksVersion` 自增广播）与 `utils/taskPlansStore.ts`
- 全局图标在 `chaos-ui/src/main.ts` 白名单注册，`meta.icon` 只能取白名单内名称

## Goals / Non-Goals

**Goals:**
- 左侧菜单出现两个并列入口「任务计划」「待办任务」，各自独立页面，无需页内切 tab
- 任务计划页保持原有全部能力与树交互行为（懒加载、展开状态保持、搜索展开、优先级 / 挂起 / 归档等）
- 待办任务页直接复用 `PendingTasks` 的表格形态，行为与拆分前 tab 内完全一致
- 两个页面之间的数据一致性由既有全局刷新信号保证，不引入新状态层

**Non-Goals:**
- 不改动后端接口、数据库模型与 `openspec/specs/task-plan/spec.md`
- 不改 `PendingTasks.vue` 的表格交互逻辑（仅移除已废弃的 `view="sidebar"` 分支与 `taskCount`/`refresh` 事件，不做功能重做）
- 不重构任务树的性能与日志（`[perf]`、`[tree]` console 输出保持原样）
- 移除侧边栏常驻的待办小列表整体（含折叠分隔标题 `sidebar-divider`、`sidebar-todo` 容器与顶部「N 待办」徽标）：待办任务只经独立「待办任务」菜单页打开，侧边栏不再承载待办入口，避免与菜单重复

## Decisions

### D1: 拆成两条顶层路由，而不是用 query / 子路由区分
`/taskPlan`（`name: taskPlan`, title「任务计划」, icon `Tickets`）与 `/pendingTask`（`name: pendingTask`, title「待办任务」, icon `Clock`）作为两条并列路由。

- 为什么不用子路由（`/task/plan`、`/task/pending`）：子路由需要父级容器路由才能生成折叠菜单，父级会多出一层「任务管理」节点，与「拆掉这个中间层」的目标相反
- 为什么不用 `?tab=`：菜单高亮与面包屑都依赖 `route.path` / `route.meta.title`，query 方案要额外处理高亮与刷新行为，收益为零
- icon 复用白名单已有图标（`Tickets` / `Clock`），避免为改菜单去动 `main.ts` 的白名单

### D2: 保留 `/task` 重定向到 `/taskPlan`
历史书签 / 外部链接可能仍指向 `/task`。加一条 `{path: '/task', redirect: '/taskPlan', meta: {hidden: true}}`，`buildMenu` 遇到 `meta.hidden` 会跳过，不会污染菜单。

### D3: `Task.vue` 一拆为二，而不是保留一个组件按路由传参
- `TaskPlan.vue`：原「任务计划」部分的完整逻辑（树加载、展开恢复、五个对话框、全部操作），删除 `activeTab` / `pendingRef` / `loading`（骨架屏随 tab 一起移除，表格已有 `v-loading`）与模板中的 tabs / 待办分支
- `PendingTask.vue`：只保留页头 + `<PendingTasks/>`，不持有业务状态

理由：两块内容的状态机几乎没有交集，按路由传参会让一个组件长期带着两套死代码；拆开后各自 `onMounted` 只加载自己需要的数据（任务计划页不再在挂载时加载待办，待办页也不再拉取整棵任务树）。

### D4: 保持「操作后全局广播」的一致性模型
原 `Task.vue` 的 `refreshAll()` 做了两件事：广播 `refreshPendingTasks()` + 直接调用页内待办实例的 `loadPendingTasks()`。拆分后页内不再有第二个待办实例，`TaskPlan.vue` 的 `refreshAll()` 只保留广播（待办页的 `PendingTasks` 实例监听 `pendingTasksVersion`，会重新拉取）。

`refreshAllPlans()` 由原来的「仅当 `activeTab === 'all'` 才拉树」简化为直接 `fetchAllPlans()`；删除失效的 `watch(activeTab)` 与 `onMounted` 里无意义的 `refreshAll()` 调用。

### D5: `searchText` 的兼容处理
`App.vue` 通过 `<component :is="Component" :search-text="searchText"/>` 向所有路由组件注入 `searchText`。`TaskPlan.vue` 继续用它做任务树搜索；`PendingTask.vue` 声明可选 props 接收但忽略，避免该属性落到根元素 DOM 上。

## Risks / Trade-offs

- 拆分后待办页不再「顺带刷新任务树」：任务计划页在路由切换时会被卸载重建，重新进入必然重新拉取；单页应用同一时刻只挂载一个路由组件，不存在需要跨页同步的窗口 → 无实际影响
- 待办页操作后通过 `pendingTasksVersion` 广播同步（如有其余常驻实例会各自重新拉取），依赖 `PendingTasks.vue` 既有的广播逻辑（完成 / 取消 / 评分 / 延期均会调用）→ 已确认覆盖全部写操作
- 纯前端改动，无数据风险；回滚只需恢复 `Task.vue` 与路由表

## Migration Plan

- 纯前端改动，无数据库 / 接口变更
- 部署：`chaos-ui` 重新 `pnpm build`，产物拷入后端静态资源目录即可（前端改动无需重编后端二进制）
- 旧路由 `/task` 由重定向兜底，用户书签不受影响
- 回滚：恢复 `views/Task.vue` 与 `router/index.ts` 原始条目，删除两个新视图文件

## Open Questions

- 「待办任务」入口图标是否需要换成更具辨识度的图标（如 `AlarmClock`）？若需要，需同步往 `main.ts` 图标白名单注册
