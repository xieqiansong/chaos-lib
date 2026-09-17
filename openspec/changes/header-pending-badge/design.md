## Context

- 前端 chaos-ui：Vue 3 + TS + Vite + Element Plus，hash 路由
- 页头在 `chaos-ui/src/App.vue` 的 `.app-header` 内，右侧已有两个终端风格按钮：主题切换（`theme-toggle`）与命令面板提示（`⌘K`）
- 待办数据源：`GET /api/tasks/pending`（不带 `early=1` 时只返回已到点任务，见 `openspec/specs/task-plan/spec.md`），前端由 `components/PendingTasks.vue` 消费，30s 轮询
- 跨组件刷新走轻量 store `utils/pendingTasksStore.ts`：`pendingTasksVersion` 自增广播，待办列表的完成 / 取消 / 延期 / 评分与中心面板复习完成都会触发
- 图标在 `chaos-ui/src/main.ts` 白名单全局注册；`Clock` 已注册（“待办任务”菜单图标），无需改白名单
- 主题变量在 `chaos-ui/src/style.css`：`--term-red` 暗色为 `#f85149`、浅黄护眼为 `#cf222e`，`--term-border` / `--term-green-faint` / `--term-green` 为终端风按钮既有取色
- 外壳 `components/TerminalFrame.vue` 的 `.term-frame__body` 为 `overflow: hidden`，页头内容超出容器会被裁切

## Goals / Non-Goals

**Goals:**

- 页头常驻可见待办数量，一眼判断是否还有到期待办
- 一处点击直达「待办任务」页面，不再经左侧菜单
- 展示形态对齐「消息未读」惯例：有未读才有红色角标，数字随数据变化即时更新
- 复用既有数据源与刷新信号，不新增后端接口、不引入新状态层

**Non-Goals:**

- 不做待办列表 / 弹窗预览，不把待办内容搬到页头（页头只做数量提示与跳转）
- 不在页头引入「提前查询」语义（角标口径固定为“已到点待办”，与待办页开关解耦）
- 不在左侧菜单项上重复挂角标（菜单入口保持原样，避免同一信息两处出现）
- 不区分逾期 / 未逾期（本次统一一个红色角标；逾期区分仍在待办页列表内）
- 不改后端接口、数据库模型与路由表

## Decisions

### D1: 独立组件 `PendingTaskBadge.vue`，而不是把逻辑写进 `App.vue`
`App.vue` 只负责布局，不持有业务数据。新增组件自带拉取、轮询、刷新信号订阅与路由跳转，`App.vue` 仅在页头插入一行 `<PendingTaskBadge/>`。

- 为什么不在 `App.vue` 里写：`App.vue` 已经有命令面板热键、主题、面包屑等职责，再塞一份待办轮询会继续膨胀，且组件无法单独调整样式
- 为什么不用 Pinia / 新建 store：数量是单一实例的展示态，用不上跨组件共享；跨页面同步已有 `pendingTasksVersion` 信号，无需新状态层

### D2: 数量口径取 `/tasks/pending` 列表长度，不新增 `count` 接口
- 后端 `GET /api/tasks/pending` 已是待办页同款联表查询，个人工具数据量小，页头 30s 一次的查询成本可忽略；新增 `?count=1` 之类的接口会同时改动 Go 侧与 spec，收益不足
- 不传 `early=1`：角标语义是“现在该做的”，与待办页「提前查询」开关解耦，避免页头数字随页内开关抖动

### D3: 刷新时机 = 挂载拉取 + 30s 轮询 + 全局信号即时刷新
- 挂载时拉一次：进入任意页面即可看到数量
- 30s 轮询兜底：读取其它实例（如移动端 / 浏览器扩展）或后台调度新生成的任务
- `watch(pendingTasksVersion)`：在待办页完成 / 取消 / 延期 / 评分、或中心面板复习完成时立刻重算，不必等下一轮轮询
- 组件挂载在 `App.vue`（非常驻路由组件之外），路由切换不销毁，计时器不会有多次注册问题；`onUnmounted` 清理计时器与 watch

### D4: 位于搜索框与主题切换按钮之间
搜索框（`Search.vue`）是页头的主要操作区，其余按钮按「待办 → 主题 → 命令面板」排列：待办是业务入口，放最靠前更醒目，同时保持现有按钮的相对顺序不变。

### D5: 角标样式沿用终端风按钮外壳 + 红色圆形角标
- 按钮整壳复用页头既有观感（透明底、`--term-border` 边框、hover 变亮），避免页头出现一块异质元素
- 角标用 `--term-red` 背景 + 白字，绝对定位在图标右上角 `-5px` 并给按钮留 6px 右外边距：既保证不被 `TerminalFrame` 的 `overflow: hidden` 裁切，也不与主题切换按钮重叠
- 无待办时不渲染角标（不是显示 0），保持页头安静；数量 >99 显示 `99+`，精确值在 `title` 悬浮提示中给出

## Risks / Trade-offs

- 页头多一次轮询请求：与待办页的轮询并行，均是只读接口；如果将来待办数据量显著增大，可改为后端 `count` 接口（本次不做）
- 数量为“已到点”口径，与打开待办页后开启「提前查询」看到的行数可能不一致 → 属于预期差异，已在 Non-Goals 中明确
- 角标为绝对定位，若后续有人给页头容器加 `overflow: hidden` 或压缩按钮间距，可能被裁切 / 重叠 → 组件内注释已写明约束来源
- 纯前端改动，无数据风险；回滚只需删除组件与 `App.vue` 中的一行引用

## Migration Plan

- 纯前端改动，无数据库 / 接口变更
- 部署：`chaos-ui` 重新 `pnpm build`，`dist/` 拷入后端静态资源目录（`scripts/chaos.deploy.ps1`），无需重编后端二进制
- 回滚：删除 `PendingTaskBadge.vue` 与 `App.vue` 中的引入 / 使用

## Open Questions

- 是否需要把「逾期待办」单独用更强烈的标识（如角标闪烁 / 红点）区分？当前统一一个红色角标，如需要再单开变更
