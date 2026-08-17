## Context

- 前端 chaos-ui 为 Vue 3 + TS + Vite + Element Plus（无 Pinia，无 vue-router 实际使用，路由用 hash + componentMap）
- 当前终端暗色主题靠两处实现：
  - `index.html` 硬编码 `<html class="dark">`，`main.ts` 全局引入 `element-plus/theme-chalk/dark/css-vars.css`（`html.dark` 生效）
  - `style.css` 中 `html.dark` 块覆盖 `--el-*`，`:root` 块定义终端自有变量 `--term-*`、字号 `--font-*`、间距 `--space-*`、大屏看板独立变量 `--board-*`
- 组件大量使用 `var(--term-*)`、`var(--el-*)`、`.text-*` 等语义类，颜色没有散落硬编码（Monaco 编辑器主题 `chaos-terminal` 是唯一硬编码色值的例外）
- `body::after` 有 CRT 扫描线 + 闪烁动画（`body.no-crt` 可关闭，`?crt=0` 或 `prefers-reduced-motion` 时生效）
- 现有 3 个未归档 change（`theme-light` / `theme-color-unify` / `terminal-typography-borders`）均为空壳，无实际 spec/design，不影响本次设计

## Goals / Non-Goals

**Goals:**
- 新增第二套主题「浅黄护眼」：暖米黄背景、深棕文字、低饱和强调色，同时覆盖 Element Plus 变量、`--term-*` 变量与 Monaco 编辑器主题
- 页头提供主题切换入口（暗色终端 ↔ 浅黄护眼），即时生效、无需刷新
- 选择持久化到 `localStorage`，默认仍为终端暗色

**Non-Goals:**
- 不做第三套以上主题（先支持两套，架构上预留扩展）
- 不做主题跟随系统 `prefers-color-scheme` 自动切换
- 不改变大屏看板（MobileBoard）的 `--board-*` 独立浅色主题（它已独立、与全局主题解耦）
- 不涉及后端 chaos-go 改动

## Decisions

### D1: 主题以 `<html>` 上的 class 为唯一开关，`dark` 为默认，新增 `paper` 表示浅黄护眼
`index.html` 移除硬编码 `class="dark"`，改为 `<html lang="zh-CN">`。运行时 JS 根据 `localStorage` 选择在 `<html>` 上挂 `dark` 或 `paper`。
- `dark`：维持现状，Element Plus dark css-vars 与 `html.dark` 覆盖块继续生效
- `paper`：Element Plus 使用默认浅色变量，再由 `html.paper` 块覆写 `--el-*` 与 `--term-*`
- 为什么不用 `light`：Element Plus 生态习惯用 `dark` 作开关（false=浅色）；用 `paper` 更能表达「护眼纸色」语义，避免与未来可能出现的普通浅色主题混淆。备选方案 `light` 会与 Element Plus 暗色开关语义（`dark` 布尔）纠缠，弃用。

### D2: 主题变量统一收敛到 `style.css`，不散落到组件
- `:root` 保留通用 token（字号/间距/`--board-*`）与 `--term-*` 默认值（终端暗色）
- 新增 `html.paper { ... }` 块：
  - 覆写全部 `--term-*`（暖米黄背景、深棕文字、橄榄绿/棕强调色、淡边框、关闭扫描线光晕）
  - 覆写 `--el-*` 背景/文字/边框/主色为护眼浅色系（Element Plus 浅色组件底色默认即可，主要调文字与主色）
  - 覆写 `--el-color-*` 语义色（success/warning/danger/info）为低饱和棕绿/琥珀/砖红
- `body` 背景与文字色已是 `var(--term-*)`，无需额外改动
- 备选方案：每组件内 `html.paper` 嵌套覆盖 → 碎片化、易漏，弃用

### D3: CRT 装饰在护眼主题下关闭
`body::after` 扫描线、闪烁与 `box-shadow` 暗角是 CRT 特效。`html.paper` 下通过 `html.paper body::after { display: none; }` 关闭；保留 `body.no-crt` 逻辑不变。护眼主题追求低干扰，扫描线会引入噪点。

### D4: 主题状态用轻量 reactive 单例（无 Pinia）
新建 `src/theme.ts`（或 `src/composables/useTheme.ts`）：模块级 `ref<'dark' | 'paper'>` + `initTheme()`（读 localStorage，无则 `dark`，应用到 `document.documentElement.classList`）+ `toggleTheme()`（切换 class、写 localStorage）。`App.vue` 页头按钮调用 `toggleTheme`，用 `@element-plus/icons-vue` 的 `Moon`/`Sunny` 图标反映当前态。
- 不引入 Pinia：项目无 Pinia 依赖，仅一个全局布尔状态，reactive 单例足够
- 备选方案：仅 App.vue 局部 ref → MonacoDiffEditor 等其他组件无法响应切换；引入 Pinia → 过度设计

### D5: MonacoDiffEditor 增加护眼主题 `chaos-paper` 并响应切换
`MonacoDiffEditor.vue` 目前 `defineTerminalTheme('chaos-terminal')` 且 `theme: 'chaos-terminal'` 固定。新增 `definePaperTheme('chaos-paper')`（`base: 'vs'`，暖米黄背景、深棕前景、低饱和语法色），并 watch 主题单例：切到 `paper` 时 `diffEditor.updateOptions({ theme: 'chaos-paper' })`，反之恢复 `chaos-terminal`。
- 备选：不做 Monaco 适配 → 护眼主题下编辑器仍黑底绿字，违背护眼目标，不做

### D6: 主题键与取值约定
`localStorage` key：`chaos-ui:theme`，取值 `'dark' | 'paper'`，非法/缺失一律回退 `dark`（首次访问默认暗色）。`initTheme` 在 `App.vue` `onMounted`（或 main.ts 挂载前）执行一次，避免 FOUC（闪白）：为减少闪烁，可在 `index.html` 内联一小段脚本优先应用 class，但作为低优先优化，首版可在 `main.ts` 顶部同步执行（早于 Vue 渲染即可）。

## Risks / Trade-offs

- 组件内少量硬编码色值（如 `TerminalFrame.vue` 标题栏 `linear-gradient(180deg, #0e150e, #0a0e0a)`、`.term-glow` 内 `rgba(51,255,102,...)`、`App.vue` 全局 `html.dark .el-table` 覆盖块）在护眼主题下可能残留暗色 → 将这些硬编码值统一改为派生变量（`--term-titlebar-grad-a/b`、`--term-glow-a/b`）或纳入 `html.paper` 覆盖
- `style.css` 中 `html.dark` 选择器写死的规则不会自动作用到 `html.paper` → 需补 `html.paper` 等价规则（主要表格文字颜色）
- ECharts 图表（dashboard）默认主题为浅色，暗色下靠 CSS 变量部分适配；护眼主题下需核对图表配色是否突兀 → 若明显不协调，可在 `html.paper` 下通过 echarts option 或 CSS 微调，列为 tasks 中人工验证项
- 主题切换只影响 `<html>` class，第三方浮层（如 ElMessage/ElNotification 的 body 级容器）跟随 class 变化，无额外处理

## Migration Plan

- 纯前端改动，无数据库/接口变更
- 部署：重新 `pnpm build` 产出 chaos-ui 静态资源，后端 embed 新前端即可
- 回滚：恢复 `index.html` 的 `class="dark"` 并回退 style.css / theme.ts 即可，无数据风险

## Open Questions

- 护眼主题的具体色值（米黄底、文字色、强调色）是否需要按用户偏好微调？→ 可在实现后人工验收时调整，不影响架构与 tasks 拆分
