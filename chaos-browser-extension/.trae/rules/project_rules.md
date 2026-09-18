# Chaos Browser Extension — 项目规则

## 项目概述
- Chrome 浏览器扩展（基于 WXT 框架 + Vue 3 + Element Plus）
- 前端入口目录：`entrypoints/`
  - `newtab/` — 新标签页（核心功能页）
  - `popup/` — 弹窗
  - `background.ts` — 后台脚本
  - `content.ts` — 内容脚本
- 后端 API 地址定义在 `utils/api.ts`，通过 `API_BASE` 导出
- 后端代码在 `chaos-lib/` 目录（文件链接），**禁止修改**

---

## 样式规范（严格遵守）

### 1. 禁止内联 `style=""`
- 所有样式必须写在 `<style scoped>` 标签内，不得在模板中使用 `style="..."` 内联样式
- 排版相关的 flex 布局可以用全局 class（见下方）

### 2. 禁止硬编码颜色值
- 不得使用 `#303133`、`#666`、`#999`、`#ebeef5`、`#fafafa`、`#ecf5ff` 等十六进制颜色
- 必须使用 Element Plus CSS 变量，对应关系如下：

| 禁止写法 | 正确写法 | 说明 |
|----------|---------|------|
| `#303133` | `var(--el-text-color-primary)` | 主文字色 |
| `#606266` / `#666` | `var(--el-text-color-regular)` | 常规文字色 |
| `#909399` / `#999` | `var(--el-text-color-secondary)` | 次要文字色 |
| `#a8abb2` | `var(--el-text-color-placeholder)` | 占位文字色 |
| `#ebeef5` | `var(--el-border-color-lighter)` | 浅边框 |
| `#dcdfe6` | `var(--el-border-color)` | 常规边框 |
| `#fafafa` | `var(--el-fill-color-lighter)` | 浅灰背景 |
| `#f0f2f5` | `var(--el-fill-color)` | 填充色 |
| `#ecf5ff` | `var(--el-color-primary-light-9)` | 浅蓝背景 |
| `#e6a23c` | `var(--el-color-warning)` | 警告色 |
| `#67c23a` | `var(--el-color-success)` | 成功色 |
| `#f56c6c` | `var(--el-color-danger)` | 危险色 |
| `#fff` / `#ffffff` | `var(--el-bg-color)` | 白色背景 |

### 3. 字体尺寸统一使用全局 class
- 定义在 `entrypoints/newtab/style.css` 中，全局可用：
  - `.text-xs` → `var(--el-font-size-extra-small)` (12px)
  - `.text-sm` → `var(--el-font-size-small)` (13px)
  - `.text-base` → `var(--el-font-size-base)` (14px)
  - `.text-md` → `var(--el-font-size-medium)` (16px)
- 模板中不得出现 `font-size: 12px` / `font-size: 14px` 等写法

### 4. 排版间距用比例
- 能用 `rem` 就用 `rem`，避免固定 `px`
- 装饰性像素值直接删除，靠 Element Plus 组件自带样式
- 可用全局间距 class：`.mb-sm`、`.mb-md`、`.mt-sm`、`.ml-sm`、`.ml-md`
- 可用全局排版 class：`.flex`、`.items-center`、`.justify-between`、`.gap-sm`、`.gap-md`、`.text-right`、`.text-center`、`.truncate`、`.font-mono`、`.flex-1`、`.min-w-0`、`.flex-shrink-0`、`.whitespace-nowrap`

### 5. 颜色语义化 class
- 定义在 `style.css` 中，全局可用：
  - `.text-primary` → `var(--el-text-color-primary)`
  - `.text-regular` → `var(--el-text-color-regular)`
  - `.text-secondary` → `var(--el-text-color-secondary)`
  - `.text-placeholder` → `var(--el-text-color-placeholder)`
  - `.bg-lighter` → `var(--el-fill-color-lighter)`
  - `.border-light` → `border: 1px solid var(--el-border-color-lighter)`

### 6. 优先使用 Element Plus 原生组件
- 状态指示器用 `el-tag`（如"未保存"/"已同步"），不要自己写 `●` + 颜色
- 卡片用 `el-card`
- 提示用 `el-alert`
- 空状态用 `el-empty`
- 能用 Element Plus 原生样式就删掉自定义样式

---

## 后端代码规则

### `chaos-lib/` 目录不可修改
- 后端代码通过文件连接（symlink/junction）引入，位于 `chaos-lib/` 目录
- `.gitignore` 中已忽略该目录
- **任何情况下都不得修改 `chaos-lib/` 中的任何文件**
- 前端通过 `utils/api.ts` 中的 `API_BASE` 与后端通信
- 开发环境：`http://localhost:8080`
- 生产环境：`http://localhost:30030`

---

## 技术栈
- **框架**: WXT (Web Extension Tools) + Vue 3 + TypeScript
- **UI 库**: Element Plus (^2.14.1)
- **浏览器 API**: webextension-polyfill
- **日期处理**: date-fns
- **构建**: `npm run dev` / `npm run build`
- **类型检查**: `npm run compile`（vue-tsc --noEmit）
- **包管理器**: pnpm（有 `pnpm-lock.yaml`，但被 gitignore）

---

## 目录结构
```
chaos-browser-extension/
├── entrypoints/
│   ├── newtab/          # 新标签页（主要功能）
│   │   ├── App.vue      # 主布局
│   │   ├── main.ts      # 入口
│   │   ├── style.css    # 全局样式 + 工具类
│   │   └── *.vue        # 各功能模块
│   ├── popup/           # 弹窗
│   ├── background.ts    # 后台脚本（API 中转）
│   └── content.ts       # 内容脚本
├── components/          # 共享组件
│   └── DiffEditor.vue   # 差异编辑器
├── utils/
│   └── api.ts           # API 地址配置
└── chaos-lib/           # 后端代码（只读，不可修改）
```