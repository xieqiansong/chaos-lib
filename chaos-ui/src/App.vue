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
import PendingTasks from './components/PendingTasks.vue'

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
  {id: 'browserHistory', label: '历史记录', children: []},
  {id: 'sdk', label: 'SDK 版本', children: []},
  {id: 'fileLink', label: '文件连接', children: []},
  {id: 'quickEdit', label: '快速编辑', children: []},
  {id: 'environment', label: '环境变量', children: []},
  {id: 'task', label: '任务管理', children: []},
  {id: 'example', label: '测试例子', children: []},
  {id: 'projectManage', label: '项目管理', children: []},
]

const activeMenuLabel = computed(() => {
  const item = menuItems.find(m => m.id === activeKey.value)
  return item?.label || ''
})

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
}

const currentComponent = computed(() => {
  return componentMap[activeKey.value] || BrowserHistory
})

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
  if (hash && menuItems.some(m => m.id === hash) && hash !== activeKey.value) {
    activeKey.value = hash
  }
}

onMounted(() => {
  window.addEventListener('hashchange', onHashChange)
  timer = setInterval(() => {
    now.value = format(new Date(), 'MM-dd HH:mm:ss')
  }, 1000)
})

onUnmounted(() => {
  clearInterval(timer)
  window.removeEventListener('hashchange', onHashChange)
})
</script>

<template>
  <div class="app-layout">
    <aside class="app-sidebar">
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
    </aside>
    <div class="app-main">
      <header class="app-header">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item></el-breadcrumb-item>
          <el-breadcrumb-item>{{ activeMenuLabel }}</el-breadcrumb-item>
        </el-breadcrumb>
        <div class="search-wrapper">
          <Search @search-change="handleSearchChange"/>
        </div>
      </header>
      <main class="app-content">
        <component :is="currentComponent" :search-text="searchText" :key="activeKey"/>
      </main>
    </div>
    <aside class="app-sidebar">
      <div class="sidebar-header">
        <span class="text-sm font-mono text-primary">待办任务</span>
        <el-tag v-if="pendingTaskCount > 0" size="small" type="primary" class="ml-sm">{{ pendingTaskCount }}</el-tag>
      </div>
      <div class="sidebar-scroll">
        <PendingTasks view="sidebar" @task-count="pendingTaskCount = $event" />
      </div>
    </aside>
  </div>
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
  border-right: 1px solid var(--el-border-color-lighter);
  border-left: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
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
  border-bottom: 1px solid var(--el-border-color-lighter);
  flex-shrink: 0;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.05rem;
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--el-border-color-lighter);
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
}

.search-wrapper {
  flex: 1;
  max-width: 66%;
  min-width: 0;
  margin-left: var(--space-lg);
}

.app-content {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-xl);
}
</style>