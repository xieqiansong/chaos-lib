<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
// @ts-ignore
import browser from 'webextension-polyfill'
import {format} from 'date-fns'
import Search from './Search.vue'

const searchText = ref('')
const activeKey = ref(window.location.hash.slice(1) || '')
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

const menuItems: MenuItem[] = []

const activeMenuLabel = computed(() => {
  const item = menuItems.find(m => m.id === activeKey.value)
  return item?.label || ''
})

const componentMap: Record<string, any> = {}

const currentComponent = computed(() => {
  return componentMap[activeKey.value] || null
})

const bookmarks = ref<any[]>([])
const bookmarkBarId = ref<string>('')

function transformBookmarks(nodes: any[]): any[] {
  return nodes.map((node: any) => ({
    ...node,
    label: node.title,
    children: node.children ? transformBookmarks(node.children) : [],
  }))
}

function loadBookmarks() {
  browser.bookmarks.getTree().then((root: any[]) => {
    const bar = (root[0]?.children || []).filter((o: any) => o.title === '书签栏')
    bookmarks.value = transformBookmarks(bar)
    if (bar.length > 0) {
      bookmarkBarId.value = bar[0].id
    }
  })
}

const defaultExpandedKeys = computed(() => {
  const keys: string[] = []
  if (bookmarkBarId.value) keys.push(bookmarkBarId.value)
  return keys
})

const treeData = computed(() => {
  return [...menuItems, ...bookmarks.value]
})

const treeProps = {
  children: 'children',
  label: 'label',
}

const handleNodeClick = (data: any) => {
  if (data.id && menuItems.some(m => m.id === data.id)) {
    activeKey.value = data.id
  } else if (data.url) {
    browser.tabs.create({url: data.url})
  }
}

watch(() => searchText.value, () => {
  loadBookmarks()
})

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
  loadBookmarks()
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
          :data="treeData"
          :props="treeProps"
          node-key="id"
          :default-expanded-keys="defaultExpandedKeys"
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
        <PendingTasks view="sidebar" @task-count="pendingTaskCount = $event"/>
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
  padding: 0 1.0rem;
  border-bottom: 1px solid var(--el-border-color-lighter);
  flex-shrink: 0;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 2.05rem;
  padding: 0 1.0rem;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
  flex-shrink: 0;
}

.sidebar-tree {
  flex: 1;
  overflow-y: auto;
  padding: 0.5rem 0;
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
  margin-left: 1rem;
}

.app-content {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
}
</style>