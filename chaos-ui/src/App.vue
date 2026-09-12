<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {format} from 'date-fns'
import Search from './views/Search.vue'
import PendingTasks from './components/PendingTasks.vue'
import TerminalFrame from './components/TerminalFrame.vue'
import CommandPalette from './components/CommandPalette.vue'
import CenterPreview from './components/CenterPreview.vue'
import ReviewDialog from './components/ReviewDialog.vue'
import {centerPanel, closeCenterPanel} from './utils/centerPanel'
import {refreshPendingTasks} from './utils/pendingTasksStore'
import {refreshTaskPlans} from './utils/taskPlansStore'
import {Moon, Sunny} from '@element-plus/icons-vue'
import {theme, toggleTheme} from './theme'
import {buildMenu, flattenMenu} from './router'

const route = useRoute()
const router = useRouter()

// 终端风格：命令面板开关 + 全局热键
const CMD_ALIAS: Record<string, string> = {
  dashboard: 'top',
  task: 'task',
  projectManage: 'proj',
  sdk: 'sdk',
  fileLink: 'link',
  quickEdit: 'edit',
  environment: 'env',
  board: 'board',
}

// 侧边菜单：完全由路由表自动生成（见 router/index.ts）
const menuItems = computed(() => buildMenu())

// 命令面板条目：菜单拍平 + shell 别名
const commandItems = computed(() => flattenMenu(menuItems.value).map(m => ({
  key: m.path,
  label: m.title,
  alias: CMD_ALIAS[m.name] || m.name,
})))

const paletteVisible = ref(false)

// 侧边栏待办区折叠开关
const todoCollapsed = ref(false)

function togglePalette() {
  paletteVisible.value = !paletteVisible.value
}

function onPaletteSelect(path: string) {
  router.push(path)
  paletteVisible.value = false
}

function onGlobalKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    togglePalette()
  }
}

// CRT 装饰开关：?crt=0 或系统减少动态效果时关闭
function applyCrtPreference() {
  const params = new URLSearchParams(window.location.search)
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (params.get('crt') === '0' || reduceMotion) {
    document.body.classList.add('no-crt')
  }
}

const searchText = ref('')
const now = ref(format(new Date(), 'MM-dd HH:mm:ss'))
let timer: ReturnType<typeof setInterval>

const pendingTaskCount = ref(0)

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

onMounted(() => {
  window.addEventListener('keydown', onGlobalKey)
  applyCrtPreference()
  timer = setInterval(() => {
    now.value = format(new Date(), 'MM-dd HH:mm:ss')
  }, 1000)
})

onUnmounted(() => {
  clearInterval(timer)
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
        <div class="sidebar-header">
          <span class="text-sm font-mono text-primary">{{ now }}</span>
          <span class="sidebar-badge" title="待办任务数量">{{ pendingTaskCount }} 待办</span>
        </div>

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

        <div class="sidebar-divider" @click="todoCollapsed = !todoCollapsed">
          <span class="term-prompt">chaos@queue:~$</span>
          <span class="sidebar-divider-label">待办任务</span>
          <span class="sidebar-divider-fold">{{ todoCollapsed ? '▸' : '▾' }}</span>
        </div>

        <div v-show="!todoCollapsed" class="sidebar-todo">
          <PendingTasks view="sidebar" @task-count="pendingTaskCount = $event"/>
        </div>
      </TerminalFrame>
    </aside>

    <div class="app-main">
      <TerminalFrame :title="currentTitle" prompt="chaos@main" hide-titlebar>
        <header class="app-header">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{path: '/dashboard'}">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-for="crumb in breadcrumbs" :key="crumb.path">
              {{ crumb.title }}
            </el-breadcrumb-item>
          </el-breadcrumb>
          <div class="search-wrapper">
            <Search @search-change="handleSearchChange"/>
          </div>
          <button
              class="cmdpalette-hint theme-toggle"
              :title="theme === 'paper' ? '切换到终端暗色' : '切换到浅黄护眼'"
              @click="toggleTheme"
          >
            <el-icon :size="14">
              <Moon v-if="theme === 'paper'"/>
              <Sunny v-else/>
            </el-icon>
          </button>
          <button class="cmdpalette-hint" title="命令面板 (Ctrl/Cmd+K)" @click="togglePalette">
            <span class="term-prompt">$</span> ⌘K
          </button>
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

  <CommandPalette
      :visible="paletteVisible"
      :commands="commandItems"
      @select="onPaletteSelect"
      @update:visible="paletteVisible = $event"
  />
</template>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.app-sidebar {
  width: calc(100vw * (300 / 1920));
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-sidebar--frame {
  padding: var(--space-xs);
}

.sidebar-nav {
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.sidebar-todo {
  flex: 1 1 0;
  min-height: 0;
  overflow-y: auto;
}

.sidebar-divider {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-xs) var(--space-lg);
  border-top: 1px solid var(--term-border);
  border-bottom: 1px solid var(--term-border);
  color: var(--term-green-faint);
  cursor: pointer;
  font-family: var(--font-mono, monospace);
  font-size: var(--font-xs);
  user-select: none;
}

.sidebar-divider:hover {
  color: var(--term-green);
  background: var(--term-active-bg);
}

.sidebar-divider-label {
  flex: 1;
}

.sidebar-divider-fold {
  flex-shrink: 0;
}

.sidebar-badge {
  margin-left: auto;
  padding: 0 var(--space-sm);
  border: 1px solid var(--term-border);
  border-radius: 2px;
  color: var(--term-green-faint);
  font-size: var(--font-xs);
  cursor: default;
}

.sidebar-badge:hover {
  color: var(--term-green);
  border-color: var(--term-green-dim);
}

.sidebar-header {
  display: flex;
  align-items: center;
  height: 2.05rem;
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--term-border);
  flex-shrink: 0;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.05rem;
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--term-border);
  background: var(--el-bg-color);
  flex-shrink: 0;
}

.sidebar-menu {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-sm) 0;
  border-right: none;
  background: transparent;
  /* 终端风格：覆盖 Element Plus 菜单变量，随主题自动切换 */
  --el-menu-bg-color: transparent;
  --el-menu-text-color: var(--term-green-faint);
  --el-menu-active-color: var(--term-green);
  --el-menu-hover-bg-color: var(--term-active-bg);
  --el-menu-hover-text-color: var(--term-green);
  --el-menu-item-height: 2rem;
  --el-menu-sub-item-height: 1.85rem;
  --el-menu-base-level-padding: var(--space-lg);
  --el-menu-level-padding: var(--space-lg);
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
  padding: var(--space-xs);
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
  height: 2.05rem;
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
  width: 1.6rem;
  height: 1.6rem;
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
  max-width: 60%;
  min-width: 0;
  margin-left: var(--space-lg);
}

.cmdpalette-hint {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: transparent;
  border: 1px solid var(--term-border);
  color: var(--term-green-faint);
  font-family: inherit;
  font-size: var(--font-xs);
  padding: 2px 8px;
  cursor: pointer;
  border-radius: 2px;
}

.cmdpalette-hint:hover {
  color: var(--term-green);
  border-color: var(--term-green-dim);
}

.app-content {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-xl);
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