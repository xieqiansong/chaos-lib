## 1. 页头待办入口组件

- [x] 1.1 新建 `chaos-ui/src/components/PendingTaskBadge.vue`：`el-icon` + `Clock` 图标按钮，样式复用页头终端风（透明底 / `--term-border` 边框 / hover 变亮）
- [x] 1.2 `PendingTaskBadge.vue`：`loadCount()` 调 `sendMessage('tasks/pending', 'GET')`（不带 `early=1`），取返回数组长度写入 `count`
- [x] 1.3 `PendingTaskBadge.vue`：`onMounted` 拉取一次 + `setInterval` 30s 轮询，`onUnmounted` 清计时器
- [x] 1.4 `PendingTaskBadge.vue`：`watch(pendingTasksVersion)` 订阅全局刷新信号，操作后即时重算；30s 轮询与信号共用同一 `loadCount`
- [x] 1.5 `PendingTaskBadge.vue`：`count > 0` 才渲染红色角标（`--term-red` 底 + 白字，右上角定位），`title` 提示「待办任务（N）」，超过 99 显示 `99+`
- [x] 1.6 `PendingTaskBadge.vue`：点击按钮 `router.push('/pendingTask')` 进入待办任务页

## 2. 页头接入

- [x] 2.1 `chaos-ui/src/App.vue`：引入 `PendingTaskBadge`，放在搜索框（`.search-wrapper`）之后、主题切换按钮（`theme-toggle`）之前
- [x] 2.2 确认未改动 `App.vue` 其余逻辑（命令面板 / 主题 / 面包屑 / 中心面板）与路由表

## 3. 构建与验收

- [x] 3.1 `chaos-ui` 构建通过（`pnpm build`）
- [ ] 3.2 前端手动验证清单（见下，用户人工验收）

## 4. 前端手动验证清单（用户人工验收）

- [ ] 4.1 页头搜索框右侧、主题切换按钮左侧出现待办入口按钮，图标为时钟；无待办时不显示红色角标
- [ ] 4.2 后端存在到期待办时，按钮右上角出现红色数字角标，数字等于待办任务页（未开「提前查询」）列表的行数
- [ ] 4.3 角标为红色，终端暗色 / 浅黄护眼两套主题下均清晰可读，且不遮挡主题切换按钮、不被外壳裁切
- [ ] 4.4 点击待办入口按钮直接跳转到「待办任务」页面（面包屑为「首页 / 待办任务」），左侧菜单「待办任务」同时高亮
- [ ] 4.5 在待办任务页点「完成 / 取消 / 延期」或提交复习评分后，页头角标数字立即变化（无需等 30s 轮询）
- [ ] 4.6 在任务计划页挂起 / 恢复 / 完成后，回到任意页面角标在 30s 内自动跟随（或刷新页面立即正确）
- [ ] 4.7 鼠标悬停按钮，提示文案为「待办任务（N）」，与角标数字一致；待办数量超过 99 时角标显示 `99+`
- [ ] 4.8 切换任意页面（看板 / 任务计划 / 项目管理等），页头按钮常驻且角标数量保持一致；连续停留数个轮询周期后无重复请求堆积、控制台无报错
