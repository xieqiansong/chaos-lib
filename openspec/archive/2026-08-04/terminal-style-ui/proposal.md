# Change Proposal: 前端改为终端命令行（Terminal）风格

## Why

当前 `chaos-ui` 使用的是 Element Plus 默认浅色/通用后台样式：常规侧边栏 + 面包屑 + 卡片式布局，视觉上与普通管理后台无异。
需求方希望整体前端具备「终端命令行 / 复古 CRT」的观感，以贴合 chaos（混沌）项目的极客定位。

目标：在**不破坏现有功能与接口**的前提下，将前端视觉语言统一为终端风格——等宽字体、暗色背景、霓虹荧光色、扫描线/栅格装饰、块边框，并提供命令面板（command palette）作为全局快捷导航。

## What Changes

- 引入全局终端主题层：CSS 变量覆盖（暗色底、荧光绿/琥珀色强调、等宽字体），扫描线（scanline）与栅格背景装饰。
- 布局保留现有三栏结构，但侧边栏/顶栏/卡片改为「终端窗口」外观：带标题栏（如 `user@chaos:~$`）、块状边框、闪烁光标装饰。
- 新增全局命令面板（Command Palette）：`Ctrl/Cmd + K` 唤起，输入命令（如 `task`、`sdk`、`ll`）即可跳转对应视图，模拟 shell 体验。
- 复用 Element Plus 组件，仅通过主题变量与少量样式覆盖实现终端观感；**不替换** Element Plus 为纯 xterm.js 渲染（避免破坏表单/表格/图表能力）。
- 图表（ECharts）配色调整为终端荧光色系，保持可读。

## 选型说明（优先流行方案）

终端风格的实现有两条主流路线：
1. **全量 xterm.js 模拟**（如 vue-web-terminal）：真实终端渲染，但会牺牲 Element Plus 表单/表格/ Monaco 编辑器能力，不适合功能型后台。
2. **主题覆盖 + 命令面板**（本项目采用）：保留 Element Plus 全部能力，仅叠加终端视觉与 shell 式交互。这是管理类应用最流行的「终端皮肤」做法，成本最低、风险最小、可维护性好。

## Impact

- 受影响文件：`chaos-ui/src/style.css`（新增终端主题变量与装饰）、`chaos-ui/src/App.vue`（布局外观 + 命令面板挂载）、各 `views/*.vue` 的局部样式、可能新增 `components/CommandPalette.vue` 与 `components/TerminalFrame.vue`。
- `package.json` 可能新增少量仅用于装饰的依赖（如 `@xterm/xterm` 仅当决定用真实终端区块时；本方案默认不引入，命令面板用纯 DOM 实现）。
- API 契约不变：所有 `/api/*` 调用、请求/响应字段、状态码保持不变。
- 不涉及后端 `chaos-go` 改动。
