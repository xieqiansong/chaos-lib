<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {Refresh, Plus, Setting, Folder, Link, FolderAdd, TopRight, MoreFilled, EditPen, Delete} from '@element-plus/icons-vue'
import {
  refreshExtStatus,
  pushExtCommand,
  getExtensionId,
  type ExtStatus,
} from '@/utils/api'

// 顶部全局搜索（与任务管理一致：由 App.vue 顶栏「请输入」输入框传入）
const props = defineProps<{
  searchText: string
}>()

const status = ref<ExtStatus | null>(null)
const treeRef = ref<any>(null)
const tree = ref<any[]>([])
const loadingTree = ref(false)
const refreshing = ref(false)

// 新建 / 新建文件夹 弹窗
const dialogVisible = ref(false)
const dialogMode = ref<'bookmark' | 'folder'>('bookmark')
const dialogParentId = ref<string | undefined>(undefined)
const formTitle = ref('')
const formUrl = ref('')

// 修改弹窗（标题 / URL）
const editVisible = ref(false)
const editNodeId = ref('')
const editIsBookmark = ref(false)
const editTitle = ref('')
const editUrl = ref('')

// el-tree 字段映射（浏览器书签树天然是嵌套结构）
const treeProps = {children: 'children', label: 'title'}

// 跳过多余的 root 根节点（id 通常为 "0"），直接展示其下的一级容器
const treeData = computed(() => {
  const t = tree.value || []
  if (t.length === 1 && t[0] && Array.isArray(t[0].children)) return t[0].children
  return t
})

// 是否文件夹（书签节点带 url，文件夹没有）
function isFolder(data: any): boolean {
  return !data?.url
}

// 默认展开「书签栏」：Chrome/Edge 中该文件夹固定为 id "1"；兜底展开第一个顶层节点
function expandBookmarksBar() {
  const t = treeData.value
  if (!t.length) return
  const bar = t.find((n: any) => n.id === '1') || t[0]
  if (bar?.id) treeRef.value?.getNode?.(bar.id)?.expand?.()
}

// 扩展直连 ID（externally_connectable）：从 localStorage 读取
const extId = ref(getExtensionId())
function saveExtId() {
  try {
    localStorage.setItem('chaos_ext_id', (extId.value || '').trim())
  } catch {
    /* ignore */
  }
  refresh()
}

async function refreshStatus() {
  try {
    status.value = await refreshExtStatus()
  } catch {
    status.value = null
  }
}

function ensureConnected(): boolean {
  if ((status.value?.connected ?? 0) <= 0) {
    ElMessage.warning('未连接扩展：请填写正确的「扩展直连 ID」并确保插件已加载')
    return false
  }
  return true
}

// 统一执行一条写指令：失败提示并返回 null
async function runCmd(cmd: any): Promise<any | null> {
  if (!ensureConnected()) return null
  try {
    const res = await pushExtCommand(cmd)
    const hit = res.response
    if (!hit || !hit.ok) {
      ElMessage.error(hit?.error || '扩展未返回结果')
      return null
    }
    return hit
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : String(e))
    return null
  }
}

// 拉取完整书签树（进入页面 / 点击刷新 / 任意写操作后调用）。silent=true 时不弹错误提示。
async function pullTree(silent = false) {
  if (!ensureConnected()) return
  loadingTree.value = true
  try {
    const res = await pushExtCommand({type: 'bookmarks:getTree', id: `tree-${Date.now()}`})
    const hit = res.response
    if (!hit || !hit.ok) {
      if (!silent) ElMessage.error('拉取书签失败：' + (hit?.error || '无响应'))
      return
    }
    tree.value = hit.echo || []
    // 默认展开「书签栏」（其余保持折叠）
    await nextTick()
    expandBookmarksBar()
  } catch (e) {
    if (!silent) ElMessage.error('拉取书签失败：' + (e instanceof Error ? e.message : String(e)))
  } finally {
    loadingTree.value = false
  }
}

// 刷新 = 刷新连接状态 + 重新拉取书签
async function refresh() {
  refreshing.value = true
  try {
    await refreshStatus()
    await pullTree()
  } finally {
    refreshing.value = false
  }
}

// 搜索书签（关键词来自顶栏搜索框）：直接筛选已加载的书签树
function doSearch() {
  const q = props.searchText.trim()
  if (!treeRef.value) return
  treeRef.value.filter(q)
  if (q) {
    // 展开全部分支，使命中节点可见
    setAllExpand(true)
  } else {
    // 清空：折叠全部，恢复默认仅展开「书签栏」
    setAllExpand(false)
    expandBookmarksBar()
  }
}

// 展开 / 折叠树中所有节点
function setAllExpand(expand: boolean) {
  const nodes = treeRef.value?.store?._getAllNodes?.() || []
  nodes.forEach((n: any) => (n.expanded = expand))
}

// el-tree 节点过滤：标题 / URL 命中即保留；父级只要任一子节点命中便自动保留
function filterNode(value: string, data: any): boolean {
  if (!value) return true
  const kw = value.toLowerCase()
  const title = (data.title || '').toLowerCase()
  const url = (data.url || '').toLowerCase()
  return title.includes(kw) || url.includes(kw)
}

// 打开书签 URL（前端直接开新标签，无需扩展）
function openNode(url?: string) {
  if (url) window.open(url, '_blank')
}

// 单击节点：文件夹切换展开/折叠，书签（带 URL）直接跳转
function onNodeClick(data: any) {
  if (data.url) {
    openNode(data.url)
    return
  }
  const node = treeRef.value?.getNode?.(data.id)
  if (node) {
    node.expanded ? node.collapse() : node.expand()
  }
}

// 更多菜单命令（修改 / 删除）
function onNodeCmd(cmd: string, data: any) {
  if (cmd === 'edit') openEdit(data)
  else if (cmd === 'delete') deleteNode(data.id, data.title, data.url)
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
  const cmd: any = {
    type: 'bookmarks:create',
    id: `create-${Date.now()}`,
    parentId: dialogParentId.value,
    title: formTitle.value.trim(),
  }
  if (dialogMode.value === 'bookmark') cmd.url = formUrl.value.trim()
  const hit = await runCmd(cmd)
  if (!hit) return
  ElMessage.success('创建成功')
  dialogVisible.value = false
  await pullTree()
}

// 修改（标题；书签还可改 URL）
function openEdit(data: any) {
  editNodeId.value = data.id
  editIsBookmark.value = !!data.url
  editTitle.value = data.title || ''
  editUrl.value = data.url || ''
  editVisible.value = true
}

async function submitEdit() {
  if (!editTitle.value.trim()) {
    ElMessage.warning('请输入标题')
    return
  }
  if (editIsBookmark.value && !editUrl.value.trim()) {
    ElMessage.warning('书签需要填写 URL')
    return
  }
  const cmd: any = {
    type: 'bookmarks:update',
    id: `update-${Date.now()}`,
    nodeId: editNodeId.value,
    title: editTitle.value.trim(),
  }
  if (editIsBookmark.value) cmd.url = editUrl.value.trim()
  const hit = await runCmd(cmd)
  if (!hit) return
  ElMessage.success('已修改')
  editVisible.value = false
  await pullTree()
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
  const hit = await runCmd({type: 'bookmarks:remove', id: `remove-${Date.now()}`, nodeId: id, isFolder: isFolder({url})})
  if (!hit) return
  ElMessage.success('已删除')
  await pullTree()
}

// ── 拖拽：修改父节点（parentId）与顺序（index）─────────────────
// 限制：只能拖进「文件夹」，或在某个文件夹内部排序；不允许改动顶层根的顺序
function allowDrop(_dragging: any, drop: any, type: 'prev' | 'inner' | 'next'): boolean {
  if (type === 'inner') return isFolder(drop.data) // 只能放进文件夹
  // prev / next 为同级排序：父级必须是真实文件夹（level===0 是浏览器根，禁止）
  return !!drop.parent && drop.parent.level > 0
}

async function onNodeDrop(dragging: any, drop: any, type: 'prev' | 'inner' | 'next') {
  const nodeId = dragging?.data?.id
  if (!nodeId) return
  const parentNode = type === 'inner' ? drop : drop.parent
  const parentId = parentNode?.data?.id
  if (!parentId) {
    await pullTree(true)
    return
  }
  // drop 后 el-tree 已把节点放入新父节点，取其在新父节点下的下标作为 index
  const index = (parentNode.childNodes || []).findIndex((n: any) => n.data?.id === nodeId)
  const hit = await runCmd({
    type: 'bookmarks:move',
    id: `move-${Date.now()}`,
    nodeId,
    parentId,
    index: index >= 0 ? index : undefined,
  })
  if (hit) ElMessage.success('已移动')
  // 无论成功与否都重新拉取，保证与浏览器书签一致
  await pullTree(true)
}

onMounted(() => {
  // 进入页面默认拉取
  refresh()
})

// 顶栏搜索词变化时即时搜索（与任务管理保持一致）
watch(() => props.searchText, () => {
  doSearch()
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">书签管理</span>
      <div class="metric-strip">
        <div class="metric-chip">
          <span class="metric-label">扩展连接</span>
          <el-tag :type="(status?.connected ?? 0) > 0 ? 'success' : 'warning'" size="small">
            {{ (status?.connected ?? 0) > 0 ? '已连接' : '未连接' }}
          </el-tag>
        </div>
        <el-popover placement="bottom-start" :width="360" trigger="click">
          <template #reference>
            <el-button size="small" text :icon="Setting">直连 ID</el-button>
          </template>
          <div style="display:flex; flex-direction:column; gap:8px;">
            <span style="font-size:13px; font-weight:500;">扩展直连 ID</span>
            <el-input
                v-model="extId"
                size="small"
                placeholder="粘贴扩展 ID（chrome://extensions 开发者模式可见）"
                @change="saveExtId"
            />
            <span style="font-size:12px; color:#909399;">填写后与浏览器扩展直连；修改后自动刷新。</span>
          </div>
        </el-popover>
      </div>
      <div class="section-actions">
        <el-button size="small" :icon="Plus" @click="openCreate(null, 'bookmark')">新建书签</el-button>
        <el-button size="small" :icon="Plus" @click="openCreate(null, 'folder')">新建文件夹</el-button>
        <el-button size="small" type="primary" :icon="Refresh" :loading="refreshing" @click="refresh">刷新</el-button>
      </div>
    </div>

    <el-alert
        v-if="(status?.connected ?? 0) <= 0"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="未连接扩展：请确认浏览器插件已加载，并点击「直连 ID」填写正确的扩展 ID"
    />

    <!-- 书签树（支持拖拽修改父节点；顶栏搜索框直接筛选树） -->
    <el-empty v-if="!treeData.length" description="暂无书签，点击「刷新」同步浏览器书签"/>
    <el-tree
        v-else
        ref="treeRef"
        :data="treeData"
        :props="treeProps"
        node-key="id"
        :indent="0"
        :expand-on-click-node="false"
        :filter-node-method="filterNode"
        draggable
        :allow-drop="allowDrop"
        @node-click="onNodeClick"
        @node-drop="onNodeDrop"
    >
        <template #default="{ data }">
          <div class="bm-node">
            <el-icon class="bm-icon" :class="data.url ? 'is-bookmark' : 'is-folder'">
              <component :is="data.url ? Link : Folder"/>
            </el-icon>
            <span class="bm-title" :class="data.url ? 'is-bookmark' : 'is-folder'">{{ data.title || '(无标题)' }}</span>
            <span class="bm-actions op-actions" @click.stop>
              <el-button v-if="data.url" text size="small" :icon="TopRight" title="打开" @click="openNode(data.url)"/>
              <el-button text size="small" :icon="Plus" title="新建书签" @click="openCreate(data.id, 'bookmark')"/>
              <el-button text size="small" :icon="FolderAdd" title="新建文件夹" @click="openCreate(data.id, 'folder')"/>
              <el-dropdown trigger="click" @command="(c) => onNodeCmd(c, data)">
                <el-button text size="small" :icon="MoreFilled" title="更多"/>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="'edit'" :icon="EditPen">修改</el-dropdown-item>
                    <el-dropdown-item :command="'delete'" :icon="Delete" class="dd-danger">删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </span>
          </div>
        </template>
      </el-tree>

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

    <!-- 修改弹窗 -->
    <el-dialog v-model="editVisible" title="修改" width="420px">
      <el-form label-width="70px">
        <el-form-item label="标题" required>
          <el-input v-model="editTitle" placeholder="标题"/>
        </el-form-item>
        <el-form-item label="URL" v-if="editIsBookmark" required>
          <el-input v-model="editUrl" placeholder="https://example.com"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="submitEdit">确定</el-button>
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

.bm-icon {
  font-size: 15px;
  flex-shrink: 0;
}

.bm-icon.is-folder {
  color: var(--el-color-warning);
}

.bm-icon.is-bookmark {
  color: var(--el-text-color-secondary);
}

.bm-title {
  white-space: nowrap;
}

.bm-title.is-folder {
  color: var(--el-text-color-primary);
  font-weight: 600;
}

.bm-title.is-bookmark {
  color: var(--el-text-color-regular);
  font-weight: 400;
}

.bm-actions {
  margin-left: auto;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.bm-node:hover .bm-actions {
  opacity: 1;
}

/* 操作按钮统一中性灰，避免大面积高饱和蓝（仅删除保留红色，见全局样式） */
.bm-actions :deep(.el-button) {
  color: var(--el-text-color-regular);
}

.bm-actions :deep(.el-button:hover) {
  color: var(--el-text-color-primary);
}

/* 层级缩进辅助线：子节点容器左内边距形成缩进，并在父级箭头处画一条竖线 */
:deep(.el-tree-node__children) {
  position: relative;
  padding-left: 16px;
}

:deep(.el-tree-node__children)::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 9px;
  border-left: 1px solid var(--el-border-color-lighter);
}
</style>

<style>
/* 下拉菜单「删除」项：仅此项保留红色（dropdown 传送到 body，scoped 无法命中） */
.el-dropdown-menu__item.dd-danger {
  color: var(--el-color-danger);
}
</style>
