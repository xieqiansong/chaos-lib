<script setup lang="ts">
import {computed, defineAsyncComponent, onUnmounted, ref, watch} from 'vue'
import {useRoute} from 'vue-router'
import Search from './views/Search.vue'
import TerminalFrame from './components/TerminalFrame.vue'
import PendingTaskBadge from './components/PendingTaskBadge.vue'
import {centerPanel, closeCenterPanel} from './utils/centerPanel'
import {refreshPendingTasks} from './utils/pendingTasksStore'
import {refreshTaskPlans} from './utils/taskPlansStore'
import {FullScreen, Moon, Sunny} from '@element-plus/icons-vue'
import {theme, toggleTheme} from './theme'
import {useLandscape} from './composables/useLandscape'
import {buildMenu} from './router'

// 中心面板（预览原文 / 复习）打开时才加载，避免 markdown-it/dompurify 常驻主包
const CenterPreview = defineAsyncComponent(() => import('./components/CenterPreview.vue'))
const ReviewDialog = defineAsyncComponent(() => import('./components/ReviewDialog.vue'))

const route = useRoute()

// 侧边菜单：完全由路由表自动生成（见 router/index.ts）
const menuItems = computed(() => buildMenu())

const paletteVisible = ref(false)

// 横屏全屏查看（仅 Android 等支持 orientation.lock 的设备显示入口）
const {locked, supported, toggle: toggleLandscape} = useLandscape()

function togglePalette() {
  paletteVisible.value = !paletteVisible.value
}

function onGlobalKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    togglePalette()
  }
}

const searchText = ref('')

const handleSearchChange = (value: string) => {
  searchText.value = value
}

// 当前路由：菜单高亮 / 标题 / 面包屑的唯一依据
const activePath = computed(() => route.path)
const currentTitle = computed(() => route.meta.title || 'main')

// 面包屑：由 route.matched 的 meta.title 自动生成（支持多级路由）
const breadcrumbs = computed(() =>
    route.matched
        .filter(r => r.meta?.title && r.path !== '/')
        .map(r => ({path: r.path, title: r.meta.title as string}))
)

// 全屏路由（大屏看板）脱离常规布局
const isFullscreen = computed(() => route.meta.fullscreen === true)

// 切换路由时收起中心面板
watch(activePath, () => closeCenterPanel())

// 中心面板（预览 / 复习）完成后刷新待办并关闭
function onCenterReviewDone() {
  refreshPendingTasks()
  refreshTaskPlans()
  closeCenterPanel()
}

onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalKey)
})
</script>

<template>
  <div v-if="isFullscreen" class="board-layout">
    <router-view/>
  </div>
  <div v-else class="app-layout">
    <aside class="app-sidebar app-sidebar--frame">
      <TerminalFrame title="nav" prompt="chaos@nav" hide-titlebar>
        <div class="sidebar-nav">
          <el-menu
              :default-active="activePath"
              class="sidebar-menu"
              router
              unique-opened
          >
            <template v-for="item in menuItems" :key="item.path">
              <el-sub-menu v-if="item.children.length" :index="item.path">
                <template #title>
                  <el-icon v-if="item.icon">
                    <component :is="item.icon"/>
                  </el-icon>
                  <span>{{ item.title }}</span>
                </template>
                <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path">
                  <el-icon v-if="child.icon">
                    <component :is="child.icon"/>
                  </el-icon>
                  <span>{{ child.title }}</span>
                </el-menu-item>
              </el-sub-menu>
              <el-menu-item v-else :index="item.path">
                <el-icon v-if="item.icon">
                  <component :is="item.icon"/>
                </el-icon>
                <span>{{ item.title }}</span>
              </el-menu-item>
            </template>
          </el-menu>
        </div>
      </TerminalFrame>
    </aside>

    <div class="app-main">
      <TerminalFrame :title="currentTitle" prompt="chaos@main" hide-titlebar>
        <header class="app-header">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{path: '/'}">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-for="crumb in breadcrumbs" :key="crumb.path">
              {{ crumb.title }}
            </el-breadcrumb-item>
          </el-breadcrumb>
          <div class="search-wrapper">
            <Search @search-change="handleSearchChange"/>
          </div>
          <div class="header-actions">
            <PendingTaskBadge/>
            <button
                class="cmdpalette-hint theme-toggle"
                :title="theme === 'paper' ? '切换到 GitHub 暗色' : '切换到 GitHub 浅色'"
                @click="toggleTheme"
            >
              <el-icon :size="16">
                <Moon v-if="theme === 'paper'"/>
                <Sunny v-else/>
              </el-icon>
            </button>
            <button
                v-if="supported"
                class="cmdpalette-hint theme-toggle"
                :title="locked ? '退出横屏全屏' : '横屏全屏查看'"
                @click="toggleLandscape"
            >
              <el-icon :size="16">
                <FullScreen/>
              </el-icon>
            </button>
          </div>
        </header>
        <main class="app-content">
          <router-view v-slot="{Component}">
            <component :is="Component" :search-text="searchText"/>
          </router-view>
        </main>
      </TerminalFrame>
      <div v-if="centerPanel" class="center-panel">
        <div class="center-panel-bar">
          <span class="center-panel-title">
            {{ centerPanel.type === 'preview' ? '预览原文' : '复习' }} — {{ centerPanel.planName }}
          </span>
          <button class="center-panel-close" title="关闭" @click="closeCenterPanel">✕</button>
        </div>
        <div class="center-panel-body">
          <CenterPreview
              v-if="centerPanel.type === 'preview'"
              :key="centerPanel.planId"
              :plan-id="centerPanel.planId"
              :plan-name="centerPanel.planName"
          />
          <ReviewDialog
              v-else
              :key="centerPanel.planId"
              :plan-id="centerPanel.planId"
              :plan-name="centerPanel.planName"
              :visible="true"
              @update:visible="closeCenterPanel"
              @done="onCenterReviewDone"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.app-sidebar {
  width: 10rem;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-sidebar--frame {
  padding: 0;
}

.sidebar-nav {
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.5rem;
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--term-border);
  background: var(--el-bg-color);
  flex-shrink: 0;
}

.sidebar-menu {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  border-right: none;
  background: transparent;
  /* 终端风格：覆盖 Element Plus 菜单变量，随主题自动切换 */
  --el-menu-bg-color: transparent;
  --el-menu-text-color: var(--term-green-faint);
  --el-menu-active-color: var(--term-green);
  --el-menu-hover-bg-color: var(--term-active-bg);
  --el-menu-hover-text-color: var(--term-green);
  --el-menu-item-height: 2.5rem;
  --el-menu-sub-item-height: 2.5rem;
  --el-menu-base-level-padding: var(--space-sm);
  --el-menu-level-padding: var(--space-sm);
}

.sidebar-menu :deep(.el-menu-item),
.sidebar-menu :deep(.el-sub-menu__title) {
  border-left: 2px solid transparent;
}

/* 当前路由高亮 */
.sidebar-menu :deep(.el-menu-item.is-active) {
  color: var(--term-green);
  background: var(--term-active-bg);
  border-left-color: var(--term-green);
}

.app-main {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  padding: 0;
}

.center-panel {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  background: var(--el-bg-color);
}

.center-panel-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.5rem;
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--term-border);
  background: var(--el-bg-color);
  flex-shrink: 0;
}

.center-panel-title {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.center-panel-close {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  background: transparent;
  border: 1px solid var(--term-border);
  color: var(--term-green-faint);
  font-family: inherit;
  font-size: var(--font-xs);
  cursor: pointer;
  border-radius: 2px;
}

.center-panel-close:hover {
  color: var(--term-green);
  border-color: var(--term-green-dim);
}

.center-panel-body {
  flex: 1;
  min-height: 0;
  padding: var(--space-xl);
  overflow: hidden;
}

.search-wrapper {
  flex: 1;
  max-width: 50%;
  min-width: 0;
  margin-left: var(--space-lg);
  display: flex;
  align-items: center;
}

/* 顶栏右侧按钮组：统一收进容器，用固定 gap 控制按钮间距，
   避免 .app-header 的 space-between 把剩余宽度均摊到按钮之间产生大空隙 */
.header-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--space-md);
  margin-left: var(--space-lg);
}

/* 顶栏按钮（主题 / 横屏 / ⌘K）：字号与内边距用大写 PX 绕过 px->rem 转换——
   手机竖屏时根字号按 100vw/120 缩到 ~3px，rem 字号会小到不可读；
   固定 PX 后与待办按钮（16px 固定图标）视觉同高。
   桌面端根字号恰为 16px，这些 PX 值与原 rem 值相等，外观零变化。 */
.cmdpalette-hint {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  background: var(--term-inner);
  border: 1px solid var(--term-border);
  color: var(--term-green-faint);
  font-family: inherit;
  font-size: var(--font-sm);
  padding: var(--space-sm);
  cursor: pointer;
  border-radius: 0.5rem;
  height: 2.4rem;
}

.cmdpalette-hint:hover {
  color: var(--term-green);
  border-color: var(--term-green-dim);
}

.app-content {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-lg);
}
</style>

<!-- 全局覆盖：确保所有 el-table 数据单元格在终端暗色主题下可读 -->
<style>
html.dark .el-table td.el-table__cell,
html.dark .el-table td.el-table__cell .cell {
  color: var(--term-green) !important;
}

html.dark .el-table th.el-table__cell,
html.dark .el-table th.el-table__cell .cell {
  color: var(--term-green-faint) !important;
}

html.paper .el-table td.el-table__cell,
html.paper .el-table td.el-table__cell .cell {
  color: var(--el-text-color-regular) !important;
}

html.paper .el-table th.el-table__cell,
html.paper .el-table th.el-table__cell .cell {
  color: var(--el-text-color-secondary) !important;
}
</style>