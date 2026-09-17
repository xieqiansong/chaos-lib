<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {Refresh, Search, Plus} from '@element-plus/icons-vue'
import {
  refreshExtStatus,
  pushExtCommand,
  getExtensionId,
  type ExtStatus,
} from '@/utils/api'

const status = ref<ExtStatus | null>(null)
const tree = ref<any[]>([])
const searchQuery = ref('')
const searchResults = ref<any[]>([])
const loadingTree = ref(false)
const searching = ref(false)
const refreshing = ref(false)

// 新建 / 新建文件夹 弹窗
const dialogVisible = ref(false)
const dialogMode = ref<'bookmark' | 'folder'>('bookmark')
const dialogParentId = ref<string | undefined>(undefined)
const formTitle = ref('')
const formUrl = ref('')

// el-tree 字段映射（浏览器书签树天然是嵌套结构）
const treeProps = {children: 'children', label: 'title'}

// 跳过多余的 root 根节点（id 通常为 "0"），直接展示其下的一级容器
const treeData = computed(() => {
  const t = tree.value || []
  if (t.length === 1 && t[0] && Array.isArray(t[0].children)) return t[0].children
  return t
})

let timer: number | undefined

// 扩展直连 ID（externally_connectable）：从 localStorage 读取，留空则回退后端中转。
const extId = ref(getExtensionId())
function saveExtId() {
  try {
    localStorage.setItem('chaos_ext_id', (extId.value || '').trim())
  } catch {
    /* ignore */
  }
  refreshStatus()
}

async function refreshStatus() {
  try {
    status.value = await refreshExtStatus()
  } catch {
    status.value = null
  }
}

async function refresh() {
  refreshing.value = true
  try {
    await refreshStatus()
  } finally {
    refreshing.value = false
  }
}

function ensureConnected(): boolean {
  if ((status.value?.connected ?? 0) <= 0) {
    ElMessage.warning('未连接扩展：请在书签管理页正确填写「扩展直连 ID」并确保插件已加载')
    return false
  }
  return true
}

// 拉取完整书签树
async function pullTree() {
  if (!ensureConnected()) return
  loadingTree.value = true
  try {
    const id = `tree-${Date.now()}`
    const res = await pushExtCommand({type: 'bookmarks:getTree', id})
    if ((res.pushed ?? 0) === 0) {
      ElMessage.warning('已下发，但当前没有已连接的扩展')
      return
    }
    const hit = res.response
    if (!hit) {
      ElMessage.error('拉取书签超时')
      return
    }
    if (!hit.ok) {
      ElMessage.error('拉取失败：' + (hit.error || ''))
      return
    }
    tree.value = hit.echo || []
    searchResults.value = []
    searchQuery.value = ''
  } finally {
    loadingTree.value = false
  }
}

// 搜索书签
async function doSearch() {
  const q = searchQuery.value.trim()
  if (!q) {
    searchResults.value = []
    return
  }
  if (!ensureConnected()) return
  searching.value = true
  try {
    const id = `search-${Date.now()}`
    const res = await pushExtCommand({type: 'bookmarks:search', query: q, id})
    if ((res.pushed ?? 0) === 0) {
      ElMessage.warning('已下发，但当前没有已连接的扩展')
      return
    }
    const hit = res.response
    if (!hit || !hit.ok) {
      ElMessage.error('搜索失败：' + (hit?.error || ''))
      return
    }
    searchResults.value = hit.echo?.nodes || []
  } finally {
    searching.value = false
  }
}

// 打开书签 URL（前端直接开新标签，无需扩展）
function openNode(url?: string) {
  if (url) window.open(url, '_blank')
}

// 新建书签 / 文件夹（parentId 为父节点 id，顶层传 undefined）
function openCreate(parentId?: string | null, mode: 'bookmark' | 'folder' = 'bookmark') {
  dialogMode.value = mode
  dialogParentId.value = parentId ?? undefined
  formTitle.value = ''
  formUrl.value = ''
  dialogVisible.value = true
}

async function submitCreate() {
  if (!formTitle.value.trim()) {
    ElMessage.warning('请输入标题')
    return
  }
  if (dialogMode.value === 'bookmark' && !formUrl.value.trim()) {
    ElMessage.warning('书签需要填写 URL')
    return
  }
  if (!ensureConnected()) return
  const id = `create-${Date.now()}`
  const cmd: any = {
    type: 'bookmarks:create',
    id,
    parentId: dialogParentId.value,
    title: formTitle.value.trim(),
  }
  if (dialogMode.value === 'bookmark') cmd.url = formUrl.value.trim()
  const res = await pushExtCommand(cmd)
  if ((res.pushed ?? 0) === 0) {
    ElMessage.warning('已下发，但当前没有已连接的扩展')
    return
  }
  const hit = res.response
  if (!hit || !hit.ok) {
    ElMessage.error('创建失败：' + (hit?.error || ''))
    return
  }
  ElMessage.success('创建成功')
  dialogVisible.value = false
  await pullTree()
}

// 重命名（书签额外询问 URL）
async function renameNode(id: string, title: string, url?: string) {
  try {
    const {value: newTitle} = await ElMessageBox.prompt('新标题', '重命名', {
      inputValue: title || '',
    })
    let newUrl = url
    if (url) {
      const {value: u} = await ElMessageBox.prompt('新 URL', '重命名书签', {
        inputValue: url || '',
      })
      newUrl = u
    }
    if (!ensureConnected()) return
    const cmdId = `update-${Date.now()}`
    const cmd: any = {type: 'bookmarks:update', id: cmdId, nodeId: id, title: newTitle}
    if (url) cmd.url = newUrl
    const res = await pushExtCommand(cmd)
    if ((res.pushed ?? 0) === 0) {
      ElMessage.warning('已下发，但当前没有已连接的扩展')
      return
    }
    const hit = res.response
    if (!hit || !hit.ok) {
      ElMessage.error('重命名失败：' + (hit?.error || ''))
      return
    }
    ElMessage.success('已重命名')
    await pullTree()
  } catch (e) {
    // 用户取消 prompt
  }
}

// 删除（文件夹用 removeTree）
async function deleteNode(id: string, title: string, url?: string) {
  try {
    await ElMessageBox.confirm(
      `确认删除「${title || url || '该项'}」？文件夹将连带删除其子项。`,
      '删除确认',
      {type: 'warning'},
    )
  } catch {
    return
  }
  if (!ensureConnected()) return
  const isFolder = !url
  const cmdId = `remove-${Date.now()}`
  const res = await pushExtCommand({type: 'bookmarks:remove', id: cmdId, nodeId: id, isFolder})
  if ((res.pushed ?? 0) === 0) {
    ElMessage.warning('已下发，但当前没有已连接的扩展')
    return
  }
  const hit = res.response
  if (!hit || !hit.ok) {
    ElMessage.error('删除失败：' + (hit?.error || ''))
    return
  }
  ElMessage.success('已删除')
  await pullTree()
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 3000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">书签管理</span>
      <div class="metric-strip">
        <div class="metric-chip">
          <span class="metric-label">已连接扩展</span>
          <el-tag :type="(status?.connected ?? 0) > 0 ? 'success' : 'warning'" size="small">
            {{ status?.connected ?? 0 }}
          </el-tag>
        </div>
      </div>
      <div class="section-actions">
        <el-button size="small" :icon="Refresh" :loading="refreshing" @click="refresh">刷新状态</el-button>
        <el-button size="small" type="primary" :loading="loadingTree" @click="pullTree">拉取书签</el-button>
      </div>
    </div>

    <el-card shadow="never" class="mb-sm">
      <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
        <span style="font-size:13px; color:#909399;">扩展直连 ID</span>
        <el-input
            v-model="extId"
            size="small"
            style="width:380px; max-width:60vw;"
            placeholder="粘贴扩展 ID（chrome://extensions 开发者模式可见）"
            @change="saveExtId"
        />
        <el-tag :type="status?.mode === 'external' ? 'success' : 'info'" size="small">
          {{ status?.mode === 'external' ? '直连模式' : '后端中转模式' }}
        </el-tag>
      </div>
      <div style="font-size:12px; color:#909399; margin-top:6px;">
        留空走后端中转；填写后与扩展直连，命令即时下发且会自动唤醒扩展（无需心跳/SSE 保活）。
      </div>
    </el-card>

    <el-alert
        v-if="(status?.connected ?? 0) <= 0"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="未连接扩展：请确认浏览器插件已加载，并在书签管理页正确填写「扩展直连 ID」"
    />

    <el-card shadow="never" class="search-card">
      <div class="search-row">
        <el-input
            v-model="searchQuery"
            class="search-input"
            placeholder="搜索书签标题 / URL"
            :prefix-icon="Search"
            clearable
            @keyup.enter="doSearch"
            @clear="searchResults = []"
        />
        <el-button :icon="Search" :loading="searching" @click="doSearch">搜索</el-button>
        <el-button :icon="Plus" @click="openCreate(null, 'bookmark')">新建书签</el-button>
        <el-button :icon="Plus" @click="openCreate(null, 'folder')">新建文件夹</el-button>
      </div>
    </el-card>

    <!-- 搜索结果 -->
    <el-card v-if="searchResults.length" shadow="never" class="tree-card">
      <template #header>
        <span class="text-primary text-sm">搜索结果（{{ searchResults.length }}）</span>
      </template>
      <el-table :data="searchResults" stripe style="width: 100%">
        <el-table-column label="标题" min-width="200" prop="title" show-overflow-tooltip/>
        <el-table-column label="URL" min-width="280" prop="url" show-overflow-tooltip/>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openNode(row.url)" v-if="row.url">打开</el-button>
            <el-button size="small" text type="warning" @click="renameNode(row.id, row.title, row.url)">重命名</el-button>
            <el-button size="small" text type="danger" @click="deleteNode(row.id, row.title, row.url)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 书签树 -->
    <el-card shadow="never" class="tree-card">
      <template #header>
        <span class="text-primary text-sm">书签树</span>
      </template>
      <el-empty v-if="!treeData.length" description="暂无书签，点击「拉取书签」同步浏览器书签"/>
      <el-tree
          v-else
          :data="treeData"
          :props="treeProps"
          node-key="id"
          :expand-on-click-node="false"
      >
        <template #default="{ data }">
          <div class="bm-node">
            <span class="bm-title">{{ data.title || '(无标题)' }}</span>
            <span v-if="data.url" class="bm-url">{{ data.url }}</span>
            <span v-else class="bm-folder">文件夹</span>
            <span class="bm-actions">
              <el-button size="small" text type="primary" @click.stop="openNode(data.url)" v-if="data.url">打开</el-button>
              <el-button size="small" text type="primary" @click.stop="openCreate(data.id, 'bookmark')">+书签</el-button>
              <el-button size="small" text type="primary" @click.stop="openCreate(data.id, 'folder')">+文件夹</el-button>
              <el-button size="small" text type="warning" @click.stop="renameNode(data.id, data.title, data.url)">重命名</el-button>
              <el-button size="small" text type="danger" @click.stop="deleteNode(data.id, data.title, data.url)">删除</el-button>
            </span>
          </div>
        </template>
      </el-tree>
    </el-card>

    <!-- 新建弹窗 -->
    <el-dialog
        v-model="dialogVisible"
        :title="dialogMode === 'bookmark' ? '新建书签' : '新建文件夹'"
        width="420px"
    >
      <el-form label-width="70px">
        <el-form-item label="标题" required>
          <el-input v-model="formTitle" placeholder="标题"/>
        </el-form-item>
        <el-form-item label="URL" v-if="dialogMode === 'bookmark'" required>
          <el-input v-model="formUrl" placeholder="https://example.com"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.metric-strip {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
}

.metric-chip {
  display: flex;
  align-items: center;
  gap: var(--space-05);
  min-width: 0;
  white-space: nowrap;
}

.metric-label {
  color: var(--el-text-color-secondary);
  font-size: var(--el-font-size-small);
  flex-shrink: 0;
}

.search-card {
  --el-card-padding: 12px;
  margin-bottom: var(--space-lg);
}

.search-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  align-items: center;
}

.search-input {
  flex: 1;
  min-width: 240px;
}

.tree-card {
  --el-card-padding: 12px;
}

.mb-sm {
  margin-bottom: var(--space-sm);
}

.bm-node {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex: 1;
  min-width: 0;
}

.bm-title {
  font-weight: 500;
  white-space: nowrap;
}

.bm-url {
  color: var(--el-text-color-secondary);
  font-size: var(--el-font-size-small);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.bm-folder {
  color: var(--el-color-warning);
  font-size: var(--el-font-size-small);
}

.bm-actions {
  display: flex;
  gap: 2px;
  margin-left: auto;
  opacity: 0.85;
}
</style>
