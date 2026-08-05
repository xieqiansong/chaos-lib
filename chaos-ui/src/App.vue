<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {format} from 'date-fns'
import Search from './views/Search.vue'
import BrowserHistory from './views/BrowserHistory.vue'
import Sdk from './views/Sdk.vue'
import FileLink from './views/FileLink.vue'
import Environment from './views/Environment.vue'
import QuickEdit from './views/QuickEdit.vue'
import Example from './views/Example.vue'
import Task from './views/Task.vue'
import Dashboard from './views/Dashboard.vue'
import ProjectManage from './views/ProjectManage.vue'
import MobileBoard from './views/MobileBoard.vue'
import PendingTasks from './components/PendingTasks.vue'
import TerminalFrame from './components/TerminalFrame.vue'
import CommandPalette from './components/CommandPalette.vue'

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

const paletteVisible = ref(false)

function togglePalette() {
  paletteVisible.value = !paletteVisible.value
}

function onPaletteSelect(key: string) {
  activeKey.value = key
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
const activeKey = ref((window.location.hash.slice(1) || 'dashboard'))
const now = ref(format(new Date(), 'MM-dd HH:mm:ss'))
let timer: ReturnType<typeof setInterval>

const pendingTaskCount = ref(0)

const handleSearchChange = (value: string) => {
  searchText.value = value
}

interface MenuItem {
  id: string
  label: string
  children: any[]
}

const menuItems: MenuItem[] = [
  {id: 'dashboard', label: '看板', children: []},
  {id: 'task', label: '任务管理', children: []},
  {id: 'projectManage', label: '项目管理', children: []},
  // {id: 'browserHistory', label: '历史记录', children: []},
  {id: 'sdk', label: 'SDK版本', children: []},
  {id: 'fileLink', label: '文件连接', children: []},
  {id: 'quickEdit', label: '快速编辑', children: []},
  {id: 'environment', label: '环境变量', children: []},
  // {id: 'example', label: '测试例子', children: []},
]

const activeMenuLabel = computed(() => {
  const item = menuItems.find(m => m.id === activeKey.value)
  return item?.label || ''
})

// 命令面板条目（基于 menuItems + shell 别名）
const commandItems = computed(() => menuItems.map(m => ({
  key: m.id,
  label: m.label,
  alias: CMD_ALIAS[m.id] || m.id,
})))

// 合法路由：菜单项 + 全屏小看板页（不显示在侧边栏）
const validKeys = [...menuItems.map(m => m.id), 'board']

const componentMap: Record<string, any> = {
  dashboard: Dashboard,
  browserHistory: BrowserHistory,
  sdk: Sdk,
  fileLink: FileLink,
  environment: Environment,
  quickEdit: QuickEdit,
  example: Example,
  task: Task,
  projectManage: ProjectManage,
  board: MobileBoard,
}

const currentComponent = computed(() => {
  return componentMap[activeKey.value] || BrowserHistory
})

// 小看板页为全屏模式，脱离常规布局
const isBoard = computed(() => activeKey.value === 'board')

const treeProps = {
  children: 'children',
  label: 'label',
}

const handleNodeClick = (data: any) => {
  if (data.id && menuItems.some(m => m.id === data.id)) {
    activeKey.value = data.id
  }
}

watch(activeKey, (val) => {
  if (window.location.hash.slice(1) !== val) {
    history.replaceState(null, '', `#${val}`)
  }
})

function onHashChange() {
  const hash = window.location.hash.slice(1)
  if (hash && validKeys.includes(hash) && hash !== activeKey.value) {
    activeKey.value = hash
  }
}

onMounted(() => {
  window.addEventListener('hashchange', onHashChange)
  window.addEventListener('keydown', onGlobalKey)
  applyCrtPreference()
  timer = setInterval(() => {
    now.value = format(new Date(), 'MM-dd HH:mm:ss')
  }, 1000)
})

onUnmounted(() => {
  clearInterval(timer)
  window.removeEventListener('hashchange', onHashChange)
  window.removeEventListener('keydown', onGlobalKey)
})
</script>

<template>
  <div v-if="isBoard" class="board-layout">
    <component :is="currentComponent" :key="activeKey"/>
  </div>
  <div v-else class="app-layout">
    <aside class="app-sidebar app-sidebar--frame">
      <TerminalFrame title="nav" prompt="chaos@nav" hide-titlebar>
        <div class="sidebar-header">
          <span class="text-sm font-mono text-primary">{{ now }}</span>
        </div>
        <el-tree
            :data="menuItems"
            :props="treeProps"
            node-key="id"
            :default-expanded-keys="[]"
            class="sidebar-tree"
            @node-click="handleNodeClick"
        />
      </TerminalFrame>
    </aside>
    <div class="app-main">
      <TerminalFrame :title="activeMenuLabel || 'main'" prompt="chaos@main" hide-titlebar>
        <header class="app-header">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item></el-breadcrumb-item>
            <el-breadcrumb-item>{{ activeMenuLabel }}</el-breadcrumb-item>
          </el-breadcrumb>
          <div class="search-wrapper">
            <Search @search-change="handleSearchChange"/>
          </div>
          <button class="cmdpalette-hint" title="命令面板 (Ctrl/Cmd+K)" @click="togglePalette">
            <span class="term-prompt">$</span> ⌘K
          </button>
        </header>
        <main class="app-content">
          <component :is="currentComponent" :search-text="searchText" :key="activeKey"/>
        </main>
      </TerminalFrame>
    </div>
    <aside class="app-sidebar app-sidebar--frame">
      <TerminalFrame title="todo" prompt="chaos@queue" hide-titlebar>
        <div class="sidebar-header">
          <span class="text-sm font-mono text-primary">待办任务</span>
          <el-tag v-if="pendingTaskCount > 0" size="small" type="primary" class="ml-sm">{{ pendingTaskCount }}</el-tag>
        </div>
        <div class="sidebar-scroll">
          <PendingTasks view="sidebar" @task-count="pendingTaskCount = $event" />
        </div>
      </TerminalFrame>
    </aside>
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

.sidebar-scroll {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
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

.sidebar-tree {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-sm) 0;
  border-right: none;
}

.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  padding: var(--space-xs);
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
  font-size: 0.72rem;
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
</style>