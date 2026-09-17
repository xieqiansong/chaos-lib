## Why

「任务管理」页面把「任务计划」和「待办任务」两块内容塞在同一个页面的两个 tab 里，该页同时承担了任务树维护（新建 / 编辑 / 挂起 / 优先级）和日常待办处理（完成 / 取消 / 延期 / 复习）两类不同节奏的操作。左侧菜单只有一个入口，日常使用时要先进入页面再切 tab，路径偏长；两块内容的加载逻辑也耦合在同一个组件中（每次操作都要同时刷新另一侧）。

## What Changes

- 左侧菜单把原「任务管理」入口拆成两个并列入口：**任务计划**、**待办任务**
- 「任务计划」页面：保留原 tab 内的任务树全部能力（新建 / 子任务 / 编辑 / 开启 / 完成 / 归档 / 挂起 / 恢复 / 优先级 / 进度 / 复习入口）
- 「待办任务」页面：保留原 tab 内的待办处理全部能力（提前查询开关、完成 / 取消 / 延期 / 预览 / 复习、逾期标记）
- 两个页面各自独立路由，页面内不再有 tab 切换
- 计划侧的增删改与状态流转继续通过全局待办刷新信号同步侧边栏与待办任务页；待办侧操作同样通过该信号同步侧边栏
- 旧地址 `/task` 重定向到 `/taskPlan`，避免历史书签失效
- 命令面板别名调整：`task` → 任务计划，新增 `todo` → 待办任务

## Capabilities

### New Capabilities
- `task-navigation`: 任务相关的导航结构，定义「任务计划」与「待办任务」两个独立入口及其职责边界

### Modified Capabilities
<!-- 无既有 spec 需求变化；task-plan spec 只约束后端接口，本次为纯前端导航结构 -->

## Impact

- `chaos-ui/src/router/index.ts`：`/task` 拆为 `/taskPlan`、`/pendingTask`，保留 `/task` 兼容重定向
- `chaos-ui/src/views/Task.vue`：拆分为 `TaskPlan.vue`（任务计划）与 `PendingTask.vue`（待办任务），原文件删除
- `chaos-ui/src/App.vue`：`CMD_ALIAS` 命令面板别名调整
- `chaos-ui/src/components/PendingTasks.vue`：无改动，继续以 `view="table"` 复用于待办任务页
- `README.md`：前端页面路由表更新
- 纯前端改动，不涉及 chaos-go 后端与数据库
