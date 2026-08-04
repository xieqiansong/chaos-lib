# Tasks

## 1. 搭建终端主题基础
- [ ] 在 `style.css` 中新增 `:root` 终端变量：覆盖 Element Plus 暗色变量（`--el-bg-color`、`--el-text-color-*`、`--el-border-color-*`、`--el-color-primary` 等）为荧光绿/黑底系，并新增 `--term-green`、`--term-amber`、`--term-red`、`--term-scanline` 等自有变量。
- [ ] 在 `main.ts`/`main.js` 引入 Element Plus 暗色主题 `dark/css-vars.css`，并给 `html` 加 `class="dark"`。
- [ ] 新增全局等宽字体栈（`font-family` 覆盖，`Consolas`/`'Courier New'` 回退，保留中文可用）。
- [ ] 新增 `.term-scanline`（扫描线）、`.term-cursor`（闪烁光标）、`.term-glow`（内发光边框）工具类；加 `@media (prefers-reduced-motion: reduce)` 关闭动画。

## 2. 终端窗口外壳组件
- [ ] 新建 `components/TerminalFrame.vue`：渲染 `user@chaos:~$ <title>` 标题栏 + 三装饰圆点 + `1px` 荧光边框 + 内发光；`title` 由 prop 传入，默认 `chaos`。
- [ ] 在 `App.vue` 中用 `TerminalFrame` 包裹左侧栏、顶栏、右侧待办栏与内容区（或整体包裹），标题随当前视图变化。

## 3. 命令面板（Command Palette）
- [ ] 新建 `components/CommandPalette.vue`：纯 DOM 实现，`$ ` 前缀输入框 + 闪烁光标 + 模糊搜索命令列表（复用 `menuItems` 标签）。
- [ ] 在 `App.vue` 挂载 `CommandPalette`，绑定全局 `Ctrl/Cmd+K` 唤起、`Esc` 关闭；选中项通过 `window.location.hash` 跳转，复用现有路由逻辑。
- [ ] 命令面板列表项显示如 `task` → 任务管理、`sdk` → SDK版本 等 shell 风格别名。

## 4. 视图配色与细节同步
- [ ] `Dashboard.vue` 图表（ECharts）`lineStyle/itemStyle/areaStyle` 改为荧光绿/琥珀系，与主题一致。
- [ ] 检查 `views/*.vue` 中硬编码颜色（如成功绿 `#67C23A`、警告 `#E6A23C`）改为终端语义变量，避免亮色突兀。
- [ ] 侧边栏在暗色/等宽下确认不溢出（保留 `truncate`、`min-w-0`）。

## 5. 构建与验证
- [ ] `pnpm build` 无错误（注意 `prebuild` 复制 monaco 资源逻辑不变）。
- [ ] 编写**前端手动验证清单**（见下），交用户人工验收。

## 前端手动验证清单
- [ ] 启动 `pnpm dev`，整体应为暗色荧光 + 等宽字体；扫描线轻微可见且不刺眼。
- [ ] 三个分区（左导航 / 顶栏 / 右待办）呈现终端窗口外观（标题栏 `user@chaos:~$` + 装饰圆点 + 边框）。
- [ ] 按 `Ctrl/Cmd+K` 弹出命令面板，输入 `task` / `sdk` / `ll` 等可模糊匹配并回车跳转对应视图。
- [ ] 各视图（看板图表、任务管理、SDK、项目、环境变量、快速编辑）功能与之前一致，无样式错乱、无文字溢出。
- [ ] ECharts 图表配色为荧光绿/琥珀，非默认蓝。
- [ ] 设置系统"减少动态效果"后刷新，扫描线/闪烁动画应关闭，布局正常。
- [ ] 收起/关闭 CRT 装饰（`?crt=0`）后界面仍可正常使用。
