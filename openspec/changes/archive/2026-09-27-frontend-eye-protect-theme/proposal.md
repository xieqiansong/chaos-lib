## Why

前端（chaos-ui）目前只有一种硬编码的终端暗色主题（CRT 荧光绿/黑底），`index.html` 写死 `class="dark"`，长时间使用暗色低对比界面容易视觉疲劳。需要一个浅黄色（米黄/护眼纸色）主题供日常使用，降低长时间盯屏的用眼负担。

## What Changes

- 新增浅黄色护眼主题（暖米黄背景 + 深棕文字 + 低饱和强调色），覆盖 Element Plus 组件变量与终端自有 CSS 变量
- 新增主题切换能力：在页头提供主题切换按钮，支持「终端暗色 / 浅黄护眼」两套主题互切
- 主题选择持久化到 `localStorage`，刷新后保持
- 移除 `index.html` 中硬编码的 `class="dark"`，改为运行时由 JS 根据持久化选择设置
- 保持终端风格组件（TerminalFrame / CommandPalette / 标题栏）在护眼主题下依然可读、视觉协调
- 默认主题仍为终端暗色，护眼主题为可选项

## Capabilities

### New Capabilities
- `theme-switcher`: 前端主题切换能力，包括多套主题定义、切换入口、持久化与运行时应用

### Modified Capabilities
<!-- 无既有 spec 需求变化 -->

## Impact

- `chaos-ui/src/style.css`：在 `html.dark` 基础上新增 `html.light`（或等价 class）主题变量块，统一管理两套变量
- `chaos-ui/index.html`：移除硬编码 `class="dark"`
- `chaos-ui/src/App.vue`：页头新增主题切换按钮，启动时读取 `localStorage` 并应用到 `<html>` class
- 使用 `--term-*` 变量的组件（TerminalFrame.vue、CommandPalette.vue、Dashboard.vue、Sdk.vue、FileLink.vue、MobileBoard.vue、App.vue 等）依赖 CSS 变量，主题化无需逐个改组件，但需核对护眼主题下的可读性
- 涉及 MonacoDiffEditor 的终端主题（`chaos-terminal`）如需跟随护眼主题，需同步定义护眼版 editor 主题
- 纯前端改动，不涉及 chaos-go 后端与数据库
