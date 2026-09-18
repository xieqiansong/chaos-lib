<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {ElMessage} from 'element-plus'
import {Folder, Refresh, Setting} from '@element-plus/icons-vue'
import BookmarkMenu from '@/components/BookmarkMenu.vue'
import {
  refreshExtStatus,
  pushExtCommand,
  getExtensionId,
  getBrowserHistories,
  searchBrowserHistories,
  type ExtStatus,
  type BrowserHistoryItem,
} from '@/utils/api'

const props = defineProps<{
  searchText?: string
}>()

// 当前搜索词（小写、去空格）；空串表示未搜索
const search = computed(() => (props.searchText || '').trim().toLowerCase())

// ── 常用书签：仿浏览器书签栏（横向展示书签栏内容，文件夹点击竖向下拉）──
const status = ref<ExtStatus | null>(null)
const bookmarksLoading = ref(false)
const bookmarksBarChildren = ref<any[]>([])
const extId = ref(getExtensionId())
// 当前在书签栏上打开下拉的文件夹 id（同一时刻仅一个）
const openFolderId = ref<string | null>(null)

// 书签栏直接子节点（含文件夹），未搜索时用于横向展示
const barChildren = computed<any[]>(() => bookmarksBarChildren.value || [])

// 搜索态：递归全树收集命中书签（标题 / URL），不限层级
const bookmarkSearchResults = computed(() => {
  const q = search.value
  if (!q) return []
  return flattenBookmarks(bookmarksBarChildren.value)
    .filter((b: any) => (b.title || '').toLowerCase().includes(q) || (b.url || '').toLowerCase().includes(q))
    .map((b: any) => ({id: b.id, title: b.title || b.url, url: b.url, favicon: favicon(b.url)}))
})

function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

function favicon(url: string): string {
  const host = hostnameOf(url)
  return host ? `https://www.google.com/s2/favicons?domain=${host}&sz=32` : ''
}

// 在书签树中定位「书签栏」节点（id "1"），兜底取第一个顶层容器
function findBookmarksBar(tree: any[]): any {
  let nodes = tree || []
  if (nodes.length === 1 && nodes[0]?.children) nodes = nodes[0].children
  return nodes.find((n: any) => n.id === '1') || nodes[0]
}

function saveExtId() {
  try {
    localStorage.setItem('chaos_ext_id', (extId.value || '').trim())
  } catch {
    /* ignore */
  }
  loadBookmarks()
}

async function loadBookmarks() {
  const st = await refreshExtStatus().catch(() => null)
  status.value = st
  openFolderId.value = null
  if ((st?.connected ?? 0) <= 0) {
    bookmarksBarChildren.value = []
    return
  }
  bookmarksLoading.value = true
  try {
    const res = await pushExtCommand({type: 'bookmarks:getTree', id: `home-tree-${Date.now()}`})
    const hit = res.response
    if (!hit || !hit.ok) {
      bookmarksBarChildren.value = []
      return
    }
    const bar = findBookmarksBar(hit.echo || [])
    bookmarksBarChildren.value = bar?.children || []
  } catch (e) {
    bookmarksBarChildren.value = []
    ElMessage.error('拉取书签失败：' + (e instanceof Error ? e.message : String(e)))
  } finally {
    bookmarksLoading.value = false
  }
}

function toggleFolder(id: string) {
  openFolderId.value = openFolderId.value === id ? null : id
}

// ── 常用书签：全部书签按浏览器访问频率排序（数据来自扩展备份的 history.VisitCount）──
const frequentLoading = ref(false)
const historyAll = ref<BrowserHistoryItem[]>([])

// 深度优先收集书签树中的全部书签节点（含各层文件夹内）
function flattenBookmarks(nodes: any[], out: any[] = []): any[] {
  for (const n of nodes || []) {
    if (n.url) out.push(n)
    if (n.children) flattenBookmarks(n.children, out)
  }
  return out
}

const frequentBookmarks = computed(() => {
  const q = search.value
  const histMap = new Map(historyAll.value.map(h => [h.Url, h]))
  let list = flattenBookmarks(bookmarksBarChildren.value)
    .map((b: any) => {
      const h = histMap.get(b.url)
      return {
        id: b.id,
        title: b.title || b.url,
        url: b.url,
        favicon: favicon(b.url),
        count: h?.VisitCount || 0,
        last: h?.LastVisitTime || 0,
      }
    })
  if (q) {
    // 搜索态：匹配全部书签（含无访问记录的），结果放宽到 100 条
    list = list.filter(b => b.title.toLowerCase().includes(q) || b.url.toLowerCase().includes(q))
  } else {
    // 常态：仅保留有访问记录的书签，取 Top 20
    list = list.filter(b => b.count > 0)
  }
  return list
    .sort((a, b) => b.count - a.count || b.last - a.last) // 频率优先，其次最近访问
    .slice(0, q ? 100 : 20)
})

async function loadFrequent() {
  frequentLoading.value = true
  try {
    historyAll.value = await getBrowserHistories()
  } catch {
    historyAll.value = []
  } finally {
    frequentLoading.value = false
  }
}

// ── 浏览器历史：最近 20 条（可滑动）──
const historyLoading = ref(false)
const histories = ref<BrowserHistoryItem[]>([])
// 搜索态：后端按关键词返回的命中结果（不限 20 条）
const searchHistories = ref<BrowserHistoryItem[]>([])

const displayHistories = computed<BrowserHistoryItem[]>(() =>
  search.value ? searchHistories.value : histories.value,
)

// Chrome lastVisitTime 为「自 1601-01-01 起的微秒数」，换算为 JS 时间戳（毫秒）
function formatHistoryTime(t: number): string {
  if (!t) return ''
  const ms = (t - 11644473600000000) / 1000
  const d = new Date(ms)
  if (isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function faviconOf(url: string): string {
  const host = hostnameOf(url)
  return host ? `https://www.google.com/s2/favicons?domain=${host}&sz=32` : ''
}

async function loadHistory() {
  historyLoading.value = true
  try {
    histories.value = await getBrowserHistories(20)
  } catch {
    histories.value = []
  } finally {
    historyLoading.value = false
  }
}

// 顶栏搜索：历史走后端关键词查询；书签/常用书签由 computed 实时递归筛选
watch(() => props.searchText, async (v) => {
  const q = (v || '').trim()
  if (q) {
    try {
      searchHistories.value = await searchBrowserHistories(q)
    } catch {
      searchHistories.value = []
    }
  } else {
    searchHistories.value = []
  }
})

onMounted(() => {
  loadBookmarks()
  loadFrequent()
  loadHistory()
})
</script>

<template>
  <div class="home">
    <!-- 常用书签 -->
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">书签栏</span>
      <span class="section-subtitle">浏览器书签栏</span>
      <div class="section-actions">
        <el-popover placement="bottom-start" :width="320" trigger="click">
          <template #reference>
            <el-button size="small" text :icon="Setting">直连 ID</el-button>
          </template>
          <div style="display:flex; flex-direction:column; gap:8px;">
            <span style="font-size:13px; font-weight:500;">扩展直连 ID</span>
            <el-input
                v-model="extId"
                size="small"
                placeholder="粘贴扩展 ID（chrome://extensions 可见）"
                @change="saveExtId"
            />
            <span style="font-size:12px; color:#909399;">填写后与浏览器扩展直连，修改后自动刷新。</span>
          </div>
        </el-popover>
        <el-button size="small" type="primary" :icon="Refresh" :loading="bookmarksLoading" @click="loadBookmarks">
          刷新
        </el-button>
      </div>
    </div>

    <el-alert
        v-if="(status?.connected ?? 0) <= 0"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="未连接扩展：请点击「直连 ID」填写正确的扩展 ID 并确保插件已加载"
    />

    <!-- 书签栏：未搜索时横向展示（文件夹点击竖向下拉）；搜索时递归全树列出命中书签 -->
    <div v-loading="bookmarksLoading">
      <div v-if="search" class="hist-list bm-search-list">
        <el-empty v-if="!bookmarkSearchResults.length" description="未找到匹配的书签"/>
        <a
            v-for="b in bookmarkSearchResults"
            :key="b.id"
            class="hist-item"
            :href="b.url"
            target="_blank"
            rel="noopener"
            :title="b.url"
        >
          <img class="hist-favicon" :src="b.favicon" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"/>
          <span class="hist-title">{{ b.title }}</span>
          <span class="hist-host">{{ hostnameOf(b.url) }}</span>
        </a>
      </div>
      <div v-else class="bm-bar">
        <el-empty v-if="!barChildren.length" description="暂无书签，点击「刷新」同步浏览器书签栏"/>
        <template v-for="node in barChildren" :key="node.id">
          <a
              v-if="node.url"
              class="bm-chip"
              :href="node.url"
              target="_blank"
              rel="noopener"
              :title="node.url"
          >
            <img class="bm-chip-fav" :src="favicon(node.url)" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"/>
            <span class="bm-chip-label">{{ node.title || node.url }}</span>
          </a>
          <div
              v-else
              class="bm-chip bm-chip-folder"
              :class="{open: openFolderId === node.id}"
              @click="toggleFolder(node.id)"
          >
            <el-icon class="bm-chip-fav"><Folder/></el-icon>
            <span class="bm-chip-label">{{ node.title || '(无标题)' }}</span>
            <span class="bm-chip-caret">▾</span>
            <div v-if="openFolderId === node.id" class="bm-dropdown" @click.stop>
              <BookmarkMenu :nodes="node.children || []"/>
            </div>
          </div>
        </template>
      </div>
    </div>
    <!-- 下拉打开时，透明遮罩拦截外部点击以关闭 -->
    <div v-if="openFolderId" class="bm-overlay" @click="openFolderId = null"/>

    <!-- 常用书签 + 浏览器历史：左右两列 -->
    <div class="panels-row">
      <!-- 常用书签：按访问频率排序 -->
      <div class="panel-col">
        <div class="section-toolbar">
          <span class="text-primary text-base section-title">常用书签</span>
          <span class="section-subtitle">按访问频率 · 最近 20 条</span>
          <div class="section-actions">
            <el-button size="small" type="primary" :icon="Refresh" :loading="frequentLoading" @click="loadFrequent">
              刷新
            </el-button>
          </div>
        </div>

        <div v-loading="frequentLoading" class="hist-list">
          <el-empty v-if="!frequentLoading && !frequentBookmarks.length" description="暂无数据：书签需要有浏览器访问记录（扩展自动备份历史）后才会出现在这里"/>
          <a
              v-for="b in frequentBookmarks"
              :key="b.id"
              class="hist-item"
              :href="b.url"
              target="_blank"
              rel="noopener"
              :title="b.url"
          >
            <img class="hist-favicon" :src="b.favicon" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"/>
            <span class="hist-title">{{ b.title }}</span>
            <span class="hist-host">{{ hostnameOf(b.url) }}</span>
            <span class="hist-count">{{ b.count }} 次</span>
            <span class="hist-time">{{ formatHistoryTime(b.last) }}</span>
          </a>
        </div>
      </div>

      <!-- 浏览器历史 -->
      <div class="panel-col">
        <div class="section-toolbar">
          <span class="text-primary text-base section-title">浏览器历史</span>
          <span class="section-subtitle">最近 20 条</span>
          <div class="section-actions">
            <el-button size="small" type="primary" :icon="Refresh" :loading="historyLoading" @click="loadHistory">
              刷新
            </el-button>
          </div>
        </div>

        <div v-loading="historyLoading" class="hist-list">
          <el-empty v-if="!historyLoading && !displayHistories.length" description="暂无历史记录，扩展会自动备份浏览器历史"/>
          <a
              v-for="h in displayHistories"
              :key="h.ID"
              class="hist-item"
              :href="h.Url"
              target="_blank"
              rel="noopener"
              :title="h.Url"
          >
            <img class="hist-favicon" :src="faviconOf(h.Url)" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"/>
            <span class="hist-title">{{ h.Title || h.Url }}</span>
            <span class="hist-host">{{ hostnameOf(h.Url) }}</span>
            <span class="hist-time">{{ formatHistoryTime(h.LastVisitTime) }}</span>
          </a>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  max-width: 100%;
}

.mb-sm {
  margin-bottom: var(--space-sm);
}

/* 常用书签 / 浏览器历史：左右两列（窄屏自动堆叠） */
.panels-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-lg);
}

.panel-col {
  flex: 1 1 22rem;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

/* 书签栏：横向换行展示 */
.bm-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: var(--space-xs);
  min-height: 1.75rem;
}

.bm-chip {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: var(--space-05);
  padding: var(--space-05) var(--space-sm);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  background: var(--el-bg-color);
  text-decoration: none;
  color: var(--el-text-color-regular);
  font-size: var(--el-font-size-small);
  max-width: 12rem;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.bm-chip:hover {
  border-color: var(--el-color-primary);
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.bm-chip-fav {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  border-radius: 2px;
  color: var(--el-color-warning);
}

.bm-chip-label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.bm-chip-caret {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-size: 10px;
  margin-left: 2px;
}

/* 文件夹下拉：竖向下挂在对应文件夹下方 */
.bm-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 20;
  min-width: 14rem;
  max-height: 20rem;
  overflow-y: auto;
  padding: var(--space-05);
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  box-shadow: var(--el-box-shadow-light);
}

.bm-overlay {
  position: fixed;
  inset: 0;
  z-index: 10;
}

/* 历史：单行紧凑列表（定高可滑动） */
.hist-list {
  display: flex;
  flex-direction: column;
  gap: 1px;
  max-height: 18rem;
  overflow-y: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  padding: var(--space-05);
}

.hist-item {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-05) var(--space-sm);
  border-radius: 3px;
  text-decoration: none;
  color: var(--el-text-color-regular);
  font-size: var(--el-font-size-small);
  transition: background 0.12s ease;
}

.hist-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.hist-favicon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  border-radius: 2px;
}

.hist-title {
  flex: 0 1 auto;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hist-host {
  flex: 1 1 0;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--el-text-color-secondary);
}

.hist-count {
  flex-shrink: 0;
  min-width: 3.5em;
  text-align: right;
  color: var(--el-text-color-secondary);
  font-variant-numeric: tabular-nums;
}

.hist-time {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-variant-numeric: tabular-nums;
}
</style>
