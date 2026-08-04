# Spec: 终端风格前端（terminal-style-ui）

## Purpose

将 `chaos-ui` 前端视觉与交互语言统一为「终端命令行 / 复古 CRT」风格，提升 chaos 项目的极客定位一致性，同时**不破坏任何现有功能、接口与后端契约**。

## Requirements

- 系统 SHALL 在 `chaos-ui` 启用暗色荧光终端主题：近黑背景、荧光绿主文字、琥珀/红为语义辅助色、全局等宽字体。
- 系统 MUST 通过覆盖 Element Plus CSS 变量实现主题，保留 Element Plus 表单/表格/图表能力，不得用 xterm.js 全量替换渲染。
- 系统 SHALL 在主要布局区域呈现「终端窗口」外观：含 `user@chaos:~$` 风格标题栏、装饰圆点、荧光边框与内发光。
- 系统 SHALL 提供全局命令面板（Command Palette），可通过 `Ctrl/Cmd+K` 唤起、`Esc` 关闭，支持模糊搜索视图别名并跳转。
- 系统 MUST 命令面板的跳转复用现有 hash 路由（`window.location.hash`），不引入新的路由机制。
- 系统 SHALL 提供 CRT 扫描线/闪烁动画装饰，且 MUST 在 `prefers-reduced-motion: reduce` 或 `?crt=0` 时关闭。
- 系统 MUST 保持所有 `/api/*` 请求、响应字段与状态码不变；不修改 `chaos-go` 后端。

## Scenario: 启用终端主题

- **WHEN** 用户打开 `chaos-ui` 根路径
- **THEN** 页面整体呈暗色荧光 + 等宽字体，主要区域带终端窗口外壳，扫描线轻微可见

## Scenario: 使用命令面板导航

- **WHEN** 用户按下 `Ctrl/Cmd+K` 并输入 `task` 后回车
- **THEN** 命令面板关闭，当前视图切换为「任务管理」，hash 变为 `#task`

## Scenario: 关闭 CRT 装饰

- **WHEN** 用户系统开启"减少动态效果"或以 `?crt=0` 访问
- **THEN** 扫描线与闪烁光标动画关闭，布局与功能保持正常

## Scenario: 功能完整性

- **WHEN** 用户在终端风格下执行原有操作（查看图表、编辑任务、切换 SDK）
- **THEN** 行为与改造前一致，无新增报错或样式错乱
