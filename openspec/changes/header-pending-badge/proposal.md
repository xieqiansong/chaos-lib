## Why

「待办任务」目前只能经左侧菜单进入（见 changes/split-task-menu），而待办数量在任何页面都不可见：页头既没有数量提示，也没有快捷入口。日常使用中（尤其是在任务计划页、快速编辑页处理完一条内容后）无法判断是否还有到期待办，容易漏掉 interval 类型的复习任务；要查看待办还得先在左侧菜单里找到入口，路径偏长。

## What Changes

- 页头（搜索框右侧、主题切换按钮左侧）新增「待办任务」入口：终端风格的图标按钮，**红色数字角标**展示当前待办数量（无待办时不显示角标，同「消息未读」的惯例）
- 点击该入口直接跳转到「待办任务」页面（`/pendingTask`）
- 数量口径与待办任务页列表一致：`GET /api/tasks/pending`（不带 `early=1`，即已到点的待处理任务），不在页头引入「提前查询」语义
- 数量同步：30s 轮询兜底；任一页面完成 / 取消 / 延期 / 评分待办（既有 `pendingTasksVersion` 全局刷新信号）后即时重算，不需要等待轮询
- 悬停提示展示「待办任务（N）」，便于角标被截断（>99 显示 `99+`）时确认精确数量
- 纯前端改动：不新增后端接口、不改数据库模型、不改路由表

## Capabilities

### New Capabilities
- `pending-task-badge`: 页头待办数量角标与快捷入口，定义待办数量的取值口径、刷新时机、展示形态（红色角标）与点击行为

### Modified Capabilities
<!-- 无既有 spec 需求变化；task-plan spec 只约束后端接口，本次为纯前端入口，未新增 / 变更接口 -->

## Impact

- 新增 `chaos-ui/src/components/PendingTaskBadge.vue`：独立组件，自带拉取 / 轮询 / 全局刷新信号订阅、角标渲染与路由跳转
- `chaos-ui/src/App.vue`：页头引入 `<PendingTaskBadge/>`（搜索框与主题切换按钮之间），不新增状态
- 复用既有 `chaos-ui/src/utils/api.ts`（`sendMessage`）与 `chaos-ui/src/utils/pendingTasksStore.ts`（`pendingTasksVersion`），不引入新状态层
- 角标颜色用主题变量 `--term-red`，暗色 / 浅黄护眼两套主题下自动跟随
- 与 `changes/split-task-menu` 的关系：该变更移除了侧边栏常驻待办小列表与侧边栏顶部「N 待办」徽标，本次把「数量提示 + 快捷入口」放回**页头**，不与菜单入口重复
- 纯前端改动，不涉及 chaos-go 后端与数据库
