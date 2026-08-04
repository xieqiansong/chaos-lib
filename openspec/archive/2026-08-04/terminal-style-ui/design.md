# Design: 终端风格前端

## 决策

1. **保留 Element Plus，仅做主题覆盖**：不引入 xterm.js 做全量渲染。Element Plus 通过 CSS 变量（`--el-color-*`、`--el-bg-color`、`--el-border-color-*`、`--el-font-size-*`）支持深度主题定制，足以把后台"染成"终端观感，同时保住表格/表单/图表能力。
2. **暗色荧光主题**：背景近黑 `#0a0e0a`，主文字荧光绿 `#33ff66`，次要文字暗绿 `#1f8f3f`，强调/警告琥珀 `#ffb000`、红 `#ff5f56`；全局 `font-family` 改为等宽 `'JetBrains Mono', 'Fira Code', Consolas, monospace`（含中文回退）。
3. **CRT 装饰层**：用 `body::after` 叠加扫描线（repeating-linear-gradient）+ 微弱 CRT 弧度阴影；低透明度、可关闭（提供 `prefers-reduced-motion` 与 `?crt=0` 关闭开关）。
4. **终端窗口外观**：抽取 `TerminalFrame.vue` 包裹内容，顶部渲染 `user@chaos:~$ <title>` 标题栏 + 三个装饰圆点（红/黄/绿，致敬 macOS 终端），内容区带 `1px` 荧光边框与内发光。
5. **命令面板（Command Palette）**：新建 `CommandPalette.vue`，`Ctrl/Cmd+K` 全局唤起，输入框带 `$ ` 前缀与闪烁光标；命令映射表复用 `App.vue` 现有 `menuItems` + `componentMap`，支持模糊搜索与回车跳转；`Esc` 关闭。
6. **导航同步**：命令面板跳转沿用现有 hash 路由（`window.location.hash`），与侧边栏点击行为一致，避免引入新路由机制。
7. **ECharts 配色同步**：在 `Dashboard.vue` 等图表处将 `lineStyle/itemStyle/areaStyle` 颜色改为荧光绿/琥珀系，保持终端一致性（颜色值用常量或 CSS 变量读取）。

## 文件计划

- `chaos-ui/src/style.css`：新增 `:root` 终端变量（覆盖 Element Plus 变量 + 自有 `--term-*`）、`@media (prefers-reduced-motion)`、`.term-scanline`、`.term-cursor` 等工具类。
- `chaos-ui/src/components/TerminalFrame.vue`：新组件，终端窗口外壳（标题栏 + 边框 + 装饰）。
- `chaos-ui/src/components/CommandPalette.vue`：新组件，全局命令面板。
- `chaos-ui/src/App.vue`：加载终端主题、用 `TerminalFrame` 包裹三栏、挂载 `CommandPalette`、绑定 `Ctrl/Cmd+K` 全局热键。
- `chaos-ui/src/views/Dashboard.vue`（及其他图表/视图）：局部配色微调以匹配荧光主题。

## 风险

- Element Plus 暗色主题需显式引入 `dark/css-vars.css` 并给 `html` 加 `class="dark"`，否则变量不生效——务必在 `main.ts`/`main.js` 处理。
- 扫描线/动画在低端机可能掉帧，须提供关闭开关且默认对 `prefers-reduced-motion` 用户关闭。
- 等宽字体下中文排版可能偏宽，需确认侧边栏/表格在窄屏不溢出（保留现有 `min-w-0` / `truncate` 约束）。
