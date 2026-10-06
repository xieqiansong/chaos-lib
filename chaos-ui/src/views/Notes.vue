<script setup lang="ts">
// 笔记主页：两栏布局 —— 目录树 | 编辑 / 预览。
// 叶节点自带 ID / Title，点文件即在右侧打开，无需中间列表中转。
// P0（只读）+ P1（读写闭环）：Monaco 编辑、保存写回、baseHash 乐观锁、新建 / 重命名 / 删除到回收站。
import {ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import NoteTree from '@/components/note/NoteTree.vue'
import NoteEditor from '@/components/note/NoteEditor.vue'
import type {Note, TreeNode} from '@/api/note'
import {noteApi} from '@/api/note'

const currentDir = ref('')

const selected = ref<Note | null>(null)
const content = ref('')
const baseHash = ref('')
const format = ref<'markdown' | 'text'>('text')

const scanning = ref(false)
const scanMsg = ref('')

const treeRef = ref<InstanceType<typeof NoteTree> | null>(null)

// 树节点（叶）→ Note：叶节点由后端索引直接派生，已带齐编辑器所需字段。
// 未由后端提供的派生字段（摘要 / 统计等）在此补零，编辑器只用 ID / Name / Title / RelPath。
function treeNodeToNote(node: TreeNode): Note {
  return {
    ID: node.id!,
    VaultID: 0,
    RelPath: node.relPath,
    ParentRel: '',
    Name: node.name,
    Title: node.title || '',
    Summary: '',
    Format: 'md',
    SizeBytes: 0,
    DiskMTime: '',
    DiskMissing: false,
    Starred: false,
    WordCount: 0,
    TagNames: '',
    IndexedAt: '',
    CreatedAt: '',
    UpdatedAt: '',
  }
}

function onSelectDir(rel: string) {
  currentDir.value = rel
}

// 点树里的笔记叶子 → 右侧直接打开。
function onOpenNote(node: TreeNode) {
  if (!node.id) {
    ElMessage.warning('该节点缺少索引 ID，请先执行「重新扫描」重建索引')
    return
  }
  const slash = node.relPath.lastIndexOf('/')
  currentDir.value = slash < 0 ? '' : node.relPath.slice(0, slash)
  openNote(treeNodeToNote(node))
}

async function openNote(note: Note) {
  selected.value = note
  try {
    const res = await noteApi.content(note.ID)
    content.value = res.content
    baseHash.value = res.contentHash
    format.value = res.format === 'md' || res.format === 'markdown' ? 'markdown' : 'text'
  } catch (e) {
    ElMessage.error('读取内容失败')
  }
}

// 编辑器保存成功：刷新本地的 hash / 内容，并刷新列表元数据（大小 / 更新时间变化）。
function onSaved(payload: { hash: string; content: string }) {
  baseHash.value = payload.hash
  treeRef.value?.load()
}

// 冲突后「重载」：重新拉取磁盘最新内容。
async function onReload() {
  if (!selected.value) return
  try {
    const res = await noteApi.content(selected.value.ID)
    content.value = res.content
    baseHash.value = res.contentHash
  } catch (e) {
    ElMessage.error('重新加载失败')
  }
}

// 重命名成功：更新当前选中项并刷新目录树。
function onRenamed(note: Note) {
  selected.value = note
  treeRef.value?.load()
}

// 删除成功：清空选中并刷新。
function onDeleted() {
  selected.value = null
  content.value = ''
  treeRef.value?.load()
}

async function doScan() {
  scanning.value = true
  scanMsg.value = ''
  try {
    const res = await noteApi.scan(false)
    scanMsg.value =
        `扫描完成：新增 ${res.added}，更新 ${res.updated}，缺失 ${res.missing}，跳过 ${res.skipped}（${res.elapsedMs}ms）`
    ElMessage.success('扫描完成')
    treeRef.value?.load()
  } catch (e) {
    ElMessage.error('扫描失败，请检查配置')
  } finally {
    scanning.value = false
  }
}

async function onCreate() {
  try {
    const {value: name} = await ElMessageBox.prompt('请输入文件名（缺扩展名默认 .md）', '新建笔记', {
      inputValidator: (v) => (v && !/[/\\]/.test(v) ? true : '文件名不能包含路径分隔符'),
    })
    const note = await noteApi.createNote(currentDir.value, name)
    ElMessage.success('已创建')
    treeRef.value?.load()
    await openNote(note)
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '创建失败')
  }
}
</script>

<template>
  <div class="notes">
    <div class="notes__toolbar">
      <span class="notes__dir">目录：{{ currentDir || '/' }}</span>
      <span class="notes__spacer"/>
      <el-button type="primary" :loading="scanning" @click="doScan">重新扫描</el-button>
      <el-button @click="onCreate">+ 新建</el-button>
      <span v-if="scanMsg" class="notes__scan-msg">{{ scanMsg }}</span>
    </div>

    <div class="notes__body">
      <div class="notes__col notes__col--tree">
        <NoteTree ref="treeRef" :current-dir="currentDir" @select-dir="onSelectDir" @open-note="onOpenNote"/>
      </div>
      <div class="notes__col notes__col--preview">
        <NoteEditor
            v-if="selected"
            :note="selected"
            :content="content"
            :base-hash="baseHash"
            :format="format"
            @saved="onSaved"
            @reload="onReload"
            @renamed="onRenamed"
            @deleted="onDeleted"
        />
        <el-empty v-else description="从左侧目录树选择笔记"/>
      </div>
    </div>
  </div>
</template>

<style scoped>
.notes {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.notes__toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 14px;
  border-bottom: 1px solid var(--el-border-color);
}

.notes__dir {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.notes__spacer {
  flex: 1;
}

.notes__scan-msg {
  color: var(--el-color-success);
  font-size: 13px;
}

.notes__body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.notes__col {
  height: 100%;
  min-height: 0;
}

.notes__col--tree {
  width: 260px;
  flex: 0 0 260px;
}

.notes__col--preview {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
</style>
