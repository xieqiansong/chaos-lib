## 1. 主题开关与持久化基础设施

- [ ] 1.1 新建 `chaos-ui/src/theme.ts`：模块级 `theme = ref<'dark' | 'paper'>`，`initTheme()` 读 `localStorage['chaos-ui:theme']`（缺失/非法回退 `dark`）并应用到 `document.documentElement.classList`（挂 `dark` 或 `paper`），`toggleTheme()` 切换 class 并写回 localStorage
- [ ] 1.2 `chaos-ui/index.html`：移除 `<html>` 上硬编码的 `class="dark"`，仅保留 `lang="zh-CN"`；在 `main.ts` 顶部（import 之后、mount 之前）同步调用 `initTheme()`，避免首屏闪烁
- [ ] 1.3 `chaos-ui/src/App.vue`：页头（`.app-header`，`⌘K` 按钮旁）新增主题切换按钮，绑定 `theme` 单例，用 `Moon`/`Sunny` 图标展示当前态，点击调用 `toggleTheme()`

## 2. 浅黄护眼主题变量

- [ ] 2.1 `chaos-ui/src/style.css`：新增 `html.paper` 块，覆写全部 `--term-*`（暖米黄背景、深棕前景、橄榄绿/琥珀/砖红低饱和强调色、淡边框、关闭扫描线/光晕派生色）
- [ ] 2.2 `html.paper` 块覆写 `--el-*`（背景/文字/边框）与 `--el-color-*`（primary/success/warning/danger/info）为护眼浅色系，保持 Element Plus 浅色组件可读
- [ ] 2.3 补充 `html.paper` 下的表格等价覆盖规则（对照现有 `html.dark .el-table ...` 块，确保护眼主题下表格文字颜色可读）
- [ ] 2.4 将组件内残留硬编码色值收敛为派生变量：`TerminalFrame.vue` 标题栏 `linear-gradient(...)` 改为 `--term-titlebar-*` 变量并补 `html.paper` 值；`.term-glow` 内 `rgba(51,255,102,...)` 改为 `--term-glow-*` 变量；`App.vue` 底部全局 `html.dark .el-table` 覆盖块如需护眼等价则补 `html.paper` 版本
- [ ] 2.5 `html.paper body::after { display: none; }` 关闭 CRT 扫描线/闪烁/暗角装饰

## 3. Monaco 编辑器护眼主题

- [ ] 3.1 `chaos-ui/src/components/MonacoDiffEditor.vue`：新增 `definePaperTheme('chaos-paper')`（`base: 'vs'`，暖米黄背景、深棕前景、低饱和语法 token 色）
- [ ] 3.2 初始化时根据当前主题选择 `theme: 'chaos-paper' | 'chaos-terminal'`，并 `watch` 主题单例：切换时 `diffEditor.updateOptions({ theme })`

## 4. 视觉验收与构建

- [ ] 4.1 `pnpm build` 通过（chaos-ui）
- [ ] 4.2 前端手动验证清单（见下）

## 5. 前端手动验证清单（用户人工验收）

- [ ] 5.1 首次访问无 localStorage 记录时，页面默认终端暗色主题，无首屏闪烁
- [ ] 5.2 点击页头主题按钮可切换到浅黄护眼主题：背景为暖米黄、文字深棕可读、无硬黑硬绿残留
- [ ] 5.3 浅黄护眼主题下逐页检查：看板（Dashboard）、任务管理、项目管理、SDK 版本、文件连接、快速编辑、环境变量，表格/树/对话框/表单/按钮均清晰可读
- [ ] 5.4 浅黄护眼主题下 `Ctrl/Cmd+K` 命令面板打开正常，提示语/高亮项可读
- [ ] 5.5 浅黄护眼主题下快速编辑页 Monaco 编辑器为浅色护眼底、语法着色可读
- [ ] 5.6 浅黄护眼主题下无 CRT 扫描线与闪烁，视觉平静
- [ ] 5.7 刷新页面后主题保持浅黄护眼；再次切换回暗色后刷新保持暗色
- [ ] 5.8 浅黄护眼主题下 ECharts 看板图表配色无明显突兀（若有则记录并微调）
