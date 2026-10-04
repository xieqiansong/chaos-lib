<script setup lang="ts">
// 笔记主页：三栏布局 —— 目录树 | 列表 | 编辑 / 预览。
// P0（只读）+ P1（读写闭环）：Monaco 编辑、保存写回、baseHash 乐观锁、新建 / 重命名 / 删除到回收站。
import {ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import NoteTree from '@/components/note/NoteTree.vue'
import NoteList from '@/components/note/NoteList.vue'
import NoteEditor from '@/components/note/NoteEditor.vue'
import type {Note} from '@/api/note'
import {noteApi} from '@/api/note'

const currentDir = ref('')
const keyword = ref('')
const onlyStarred = ref(false)
const onlyMissing = ref(false)

const selected = ref<Note | null>(null)
const content = ref('')
const baseHash = ref('')
const format = ref<'markdown' | 'text'>('text')
const loadingContent = ref(false)

const scanning = ref(false)
const scanMsg = ref('')

const treeRef = ref<InstanceType<typeof NoteTree> | null>(null)
const listRef = ref<InstanceType<typeof NoteList> | null>(null)

function parentOf(rel: string): string {
  const trimmed = rel.replace(/\/$/, '')
  const i = trimmed.lastIndexOf('/')
  return i < 0 ? '' : trimmed.slice(0, i)
}

let pendingOpenRel = ''

function onSelectDir(rel: string) {
  currentDir.value = rel
  pendingOpenRel = ''
}

function onOpenNote(rel: string) {
  // 树里的笔记叶子：切到其所在目录，列表加载后自动打开。
  currentDir.value = parentOf(rel)
  pendingOpenRel = rel
}

function onListLoaded(rows: Note[]) {
  if (pendingOpenRel) {
    const hit = rows.find((r) => r.RelPath === pendingOpenRel)
    if (hit) {
      openNote(hit)
      pendingOpenRel = ''
      return
    }
  }
}

async function openNote(note: Note) {
  selected.value = note
  loadingContent.value = true
  try {
    const res = await noteApi.content(note.ID)
    content.value = res.content
    baseHash.value = res.contentHash
    format.value = res.format === 'md' || res.format === 'markdown' ? 'markdown' : 'text'
  } catch (e) {
    ElMessage.error('读取内容失败')
  } finally {
    loadingContent.value = false
  }
}

// 编辑器保存成功：刷新本地的 hash / 内容，并刷新列表元数据（大小 / 更新时间变化）。
function onSaved(payload: { hash: string; content: string }) {
  content.value = payload.content
  baseHash.value = payload.hash
  listRef.value?.load()
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

// 重命名成功：更新当前选中项并刷新目录树 / 列表。
function onRenamed(note: Note) {
  selected.value = note
  treeRef.value?.load()
  listRef.value?.load()
}

// 删除成功：清空选中并刷新。
function onDeleted() {
  selected.value = null
  content.value = ''
  treeRef.value?.load()
  listRef.value?.load()
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
    listRef.value?.load()
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
    listRef.value?.load()
    await openNote(note)
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '创建失败')
  }
}
</script>

<template>
  <div class="notes">
    <div class="notes__toolbar">
      <el-input
          v-model="keyword"
          placeholder="搜索标题 / 正文"
          clearable
          style="width: 240px"
      />
      <el-switch v-model="onlyStarred" active-text="星标"/>
      <el-switch v-model="onlyMissing" active-text="缺失"/>
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
      <div class="notes__col notes__col--list">
        <NoteList
            ref="listRef"
            :current-dir="currentDir"
            :keyword="keyword"
            :only-starred="onlyStarred"
            :only-missing="onlyMissing"
            @open="openNote"
            @loaded="onListLoaded"
        />
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
        <el-empty v-else description="选择左侧笔记进行编辑"/>
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
  padding: 10px 14px;
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

.notes__col--list {
  width: 420px;
  flex: 0 0 420px;
}

.notes__col--preview {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
</style>
