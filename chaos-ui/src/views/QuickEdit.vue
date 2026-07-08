<script setup lang="ts">
import {computed, defineAsyncComponent, onMounted, ref, watch} from 'vue'
import {sendMessage} from '@/utils/api'
const MonacoDiffEditor = defineAsyncComponent(() => import('../components/MonacoDiffEditor.vue'))

const props = defineProps<{
  searchText: string
}>()

interface QuickEditFile {
  ID: number
  Name: string
  FilePath: string
  Remark: string
  CreatedAt: string
  UpdatedAt: string
  LastSnapshotID: number
  LastSnapshotTime: string
}

interface QuickEditSnapshot {
  ID: number
  FileID: number
  SizeBytes: number
  CreatedAt: string
}

const files = ref<QuickEditFile[]>([])
const error = ref('')
const loading = ref(false)

const showCreateModal = ref(false)
const newFile = ref({
  Name: '',
  FilePath: '',
  Remark: ''
})

const activeFileId = ref<number | null>(null)
const content = ref('')
const originalContent = ref('')
const isDirty = ref(false)
const saving = ref(false)
const contentLoading = ref(false)


const snapshots = ref<QuickEditSnapshot[]>([])
const snapshotPage = ref(1)
const snapshotTotal = ref(0)
const snapshotLoading = ref(false)
const selectedSnapshotId = ref<number | null>(null)
const snapshotContent = ref('')
const snapshotContentLoading = ref(false)

async function fetchFiles() {
  loading.value = true
  error.value = ''
  try {
    const result = await sendMessage('quickEdits', 'GET')
    if (Array.isArray(result)) {
      files.value = result
    } else if (result && Array.isArray(result.data)) {
      files.value = result.data
    } else {
      files.value = []
    }
    autoSelectLatest()
  } catch (e) {
    error.value = '获取文件列表失败'
    console.error(e)
  } finally {
    loading.value = false
  }
}

function autoSelectLatest() {
  if (files.value.length === 0) return
  const latest = files.value.reduce((a, b) => {
    const ta = new Date(a.UpdatedAt || a.CreatedAt).getTime()
    const tb = new Date(b.UpdatedAt || b.CreatedAt).getTime()
    return ta > tb ? a : b
  })
  if (activeFileId.value !== latest.ID) {
    openFile(latest)
  }
}

async function createFile() {
  if (!newFile.value.FilePath.trim()) {
    error.value = '请填写文件路径'
    return
  }
  try {
    const result = await sendMessage('quickEdits', 'POST', {
      name: newFile.value.Name,
      filePath: newFile.value.FilePath,
      remark: newFile.value.Remark
    })
    if (result && result.error) {
      error.value = result.error || '创建失败'
      return
    }
    showCreateModal.value = false
    newFile.value = {Name: '', FilePath: '', Remark: ''}
    await fetchFiles()
  } catch (e: any) {
    error.value = e?.message || '创建文件失败'
    console.error(e)
  }
}

async function deleteFile(id: number) {
  try {
    const result = await sendMessage(`quickEdits/${id}`, 'DELETE')
    if (result && result.error) {
      error.value = result.error || '删除失败'
      return
    }
    if (activeFileId.value === id) {
      activeFileId.value = null
      content.value = ''
      originalContent.value = ''
      isDirty.value = false
    }
    await fetchFiles()
  } catch (e) {
    error.value = '删除失败'
    console.error(e)
  }
}

async function openFile(file: QuickEditFile) {
  if (activeFileId.value === file.ID) return
  activeFileId.value = file.ID
  contentLoading.value = true
  error.value = ''
  try {
    const result = await sendMessage(`quickEdits/${file.ID}/content`, 'GET')
    if (result && result.content !== undefined) {
      content.value = result.content
      originalContent.value = result.content
      isDirty.value = false
    } else {
      error.value = '获取文件内容失败'
    }
    // 同时加载快照列表
    snapshotPage.value = 1
    snapshots.value = []
    snapshotTotal.value = 0
    selectedSnapshotId.value = null
    snapshotContent.value = ''
    await fetchSnapshots()
  } catch (e) {
    error.value = '获取文件内容失败'
    console.error(e)
  } finally {
    contentLoading.value = false
  }
}

function handleContentChange(val: string) {
  content.value = val
  isDirty.value = val !== originalContent.value
  if (error.value) error.value = ''
}

async function saveContent() {
  if (!isDirty.value || activeFileId.value === null) return
  saving.value = true
  error.value = ''
  try {
    const result = await sendMessage(`quickEdits/${activeFileId.value}/content`, 'PUT', {
      content: content.value
    })
    if (result && !result.error) {
      originalContent.value = content.value
      isDirty.value = false
      await fetchFiles()
    } else {
      error.value = result?.error || '保存失败'
    }
  } catch (e) {
    error.value = '保存失败'
    console.error(e)
  } finally {
    saving.value = false
  }
}

async function resetContent() {
  if (!isDirty.value) return
  content.value = originalContent.value
  isDirty.value = false
  error.value = ''
}


async function fetchSnapshots() {
  if (activeFileId.value === null) return
  snapshotLoading.value = true
  try {
    const result = await sendMessage(
        `quickEdits/${activeFileId.value}/snapshots`,
        'GET',
        {page: snapshotPage.value, size: 20}
    )
    if (result && Array.isArray(result.items)) {
      snapshots.value = result.items
      snapshotTotal.value = result.total || 0
    } else if (Array.isArray(result)) {
      snapshots.value = result
    }
    if (snapshots.value.length > 0 && selectedSnapshotId.value === null) {
      viewSnapshot(snapshots.value[0])
    }
  } catch (e) {
    error.value = '获取快照失败'
    console.error(e)
  } finally {
    snapshotLoading.value = false
  }
}

async function viewSnapshot(snap: QuickEditSnapshot) {
  if (activeFileId.value === null) return
  selectedSnapshotId.value = snap.ID
  snapshotContentLoading.value = true
  try {
    const result = await sendMessage(
        `quickEdits/${activeFileId.value}/snapshots/${snap.ID}`,
        'GET'
    )
    if (result && result.content !== undefined) {
      snapshotContent.value = result.content
    } else {
      snapshotContent.value = ''
    }
  } catch (e) {
    console.error(e)
    snapshotContent.value = ''
  } finally {
    snapshotContentLoading.value = false
  }
}

function onSnapshotSelect(snapshotId: number) {
  const snap = snapshots.value.find(s => s.ID === snapshotId)
  if (snap) {
    viewSnapshot(snap)
  }
}

async function restoreSnapshot() {
  if (activeFileId.value === null || selectedSnapshotId.value === null) return
  try {
    const result = await sendMessage(
        `quickEdits/${activeFileId.value}/restore`,
        'POST',
        {snapshotId: selectedSnapshotId.value}
    )
    if (result && !result.error) {
      activeFileId.value = null
      await fetchFiles()
    } else {
      error.value = result?.error || '回滚失败'
    }
  } catch (e) {
    error.value = '回滚失败'
    console.error(e)
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  try {
    const d = new Date(t)
    return d.toLocaleString('zh-CN', {hour12: false})
  } catch {
    return t
  }
}

const activeFile = computed(() =>
    files.value.find((f) => f.ID === activeFileId.value) || null
)

const diffOriginal = computed(() =>
    selectedSnapshotId.value ? snapshotContent.value : originalContent.value
)

const codeLanguage = computed(() => {
  const name = activeFile.value?.Name || activeFile.value?.FilePath || ''
  const ext = name.split('.').pop()?.toLowerCase()
  const map: Record<string, string> = {
    js: 'javascript', ts: 'typescript', vue: 'html', jsx: 'javascript', tsx: 'typescript',
    json: 'json', xml: 'xml', html: 'html', css: 'css', scss: 'scss', less: 'less',
    py: 'python', go: 'go', rs: 'rust', java: 'java', c: 'c', cpp: 'cpp', h: 'c',
    sh: 'bash', bash: 'bash', zsh: 'bash', yml: 'yaml', yaml: 'yaml', toml: 'ini',
    md: 'markdown', sql: 'sql', php: 'php', rb: 'ruby', swift: 'swift', kt: 'kotlin',
    dockerfile: 'dockerfile', makefile: 'makefile',
  }
  return ext && map[ext] ? map[ext] : 'plaintext'
})

const filteredFiles = computed(() => {
  const q = props.searchText.trim().toLowerCase()
  if (!q) return files.value
  return files.value.filter(
      (f) =>
          f.Name.toLowerCase().includes(q) ||
          f.FilePath.toLowerCase().includes(q) ||
          f.Remark.toLowerCase().includes(q)
  )
})

watch(() => props.searchText, () => {
  // 仅用于过滤，不重新拉取
})

onMounted(() => {
  fetchFiles()
})
</script>

<template>
  <div>
    <div class="quickedit-toolbar">
      <span class="text-primary text-base quickedit-title">快速编辑</span>
      <el-skeleton v-if="loading" :rows="1" animated class="quickedit-loading"/>
      <template v-else-if="filteredFiles.length > 0">
        <el-tag
            v-for="file in filteredFiles"
            :key="file.ID"
            :type="activeFileId === file.ID ? 'primary' : 'info'"
            closable
            size="default"
            class="quickedit-file-tag"
            @click="openFile(file)"
            @close="deleteFile(file.ID)"
        >
          {{ file.Name }}
        </el-tag>
      </template>
      <span v-else class="text-secondary text-xs">暂无登记文件</span>
      <div class="quickedit-actions">
        <el-button size="small" @click="fetchFiles" :loading="loading" plain>刷新</el-button>
        <el-button size="small" type="primary" @click="showCreateModal = true">+ 登记</el-button>
      </div>
    </div>

    <el-alert
        v-if="error"
        type="error"
        :message="error"
        show-icon
        class="mb-sm"
        @close="error = ''"
    />

    <el-row :gutter="16">
      <el-col :span="24">
        <div v-if="activeFile === null" class="quickedit-empty">
          <el-empty description="从上方选择一个文件进行编辑"/>
        </div>
        <div v-else class="quickedit-editor-wrap">
          <div class="quickedit-editor-header">
            <div class="quickedit-snapshot-bar">
              <span class="text-sm quickedit-section-label">历史快照</span>
              <el-select
                  v-model="selectedSnapshotId"
                  placeholder="选择快照"
                  filterable
                  size="small"
                  class="quickedit-snapshot-select"
                  @change="onSnapshotSelect"
                  :loading="snapshotLoading"
                  clearable
              >
                <el-option
                    v-for="snap in snapshots"
                    :key="snap.ID"
                    :label="`#${snap.ID} · ${formatTime(snap.CreatedAt)}`"
                    :value="snap.ID"
                />
              </el-select>
              <div v-if="snapshotTotal > 20" class="flex items-center gap-sm">
                <el-button size="small" :disabled="snapshotPage <= 1" @click="snapshotPage--; fetchSnapshots()" class="quickedit-page-btn">‹</el-button>
                <span class="text-secondary text-xs">{{ snapshotPage }}/{{ Math.ceil(snapshotTotal / 20) }}</span>
                <el-button size="small" :disabled="snapshotPage >= Math.ceil(snapshotTotal / 20)" @click="snapshotPage++; fetchSnapshots()"
                           class="quickedit-page-btn">›
                </el-button>
              </div>
              <el-button size="small" @click="fetchSnapshots" :loading="snapshotLoading">刷新</el-button>
              <el-button type="warning" size="small" :disabled="selectedSnapshotId === null" @click="restoreSnapshot">回滚到此版本</el-button>
            </div>
            <div class="quickedit-file-info">
              <div class="flex-1 min-w-0">
                <div class="text-sm truncate quickedit-file-name">{{ activeFile.Name }}</div>
                <div class="text-secondary text-xs truncate mt-sm">{{ activeFile.FilePath }}</div>
              </div>
              <div class="quickedit-file-actions">
                <el-button type="info" size="small" @click="resetContent" :disabled="!isDirty">还原</el-button>
                <el-button type="primary" size="small" @click="saveContent" :loading="saving" :disabled="!isDirty">保存</el-button>
              </div>
            </div>
          </div>

          <div class="quickedit-diff-container">
            <el-skeleton v-if="contentLoading || snapshotContentLoading" :rows="15" animated class="quickedit-diff-skeleton"/>
            <MonacoDiffEditor
                v-else
                v-model="content"
                :original="diffOriginal"
                :language="codeLanguage"
                @update:model-value="handleContentChange"
            />
          </div>
          <div class="quickedit-status-bar">
            <el-tag v-if="isDirty" type="warning" size="small">未保存的更改</el-tag>
            <el-tag v-else type="success" size="small">已同步</el-tag>
            <span class="text-secondary text-xs ml-sm">字符数: {{ content.length }}</span>
          </div>
        </div>
      </el-col>
    </el-row>

    <el-dialog v-model="showCreateModal" title="登记文件" width="600px">
      <el-form :model="newFile" label-width="100px">
        <el-form-item label="文件路径" required>
          <el-input
              v-model="newFile.FilePath"
              placeholder="例如 C:\Users\me\config.json 或 /etc/nginx/nginx.conf"/>
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="newFile.Name" placeholder="可选，留空自动使用文件名"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input
              v-model="newFile.Remark"
              type="textarea"
              :rows="2"
              placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateModal = false">取消</el-button>
        <el-button type="primary" @click="createFile">登记</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.quickedit-toolbar {
  margin-bottom: var(--space-md);
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

.quickedit-title {
  font-weight: 600;
  white-space: nowrap;
}

.quickedit-loading {
  width: 7.5rem;
}

.quickedit-file-tag {
  cursor: pointer;
}

.quickedit-actions {
  margin-left: auto;
  display: flex;
  gap: var(--space-05);
}

.quickedit-empty {
  border: 1px dashed var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  padding: 5rem 0;
  text-align: center;
  color: var(--el-text-color-secondary);
}

.quickedit-editor-wrap {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  background: var(--el-bg-color);
  overflow: hidden;
}

.quickedit-editor-header {
  display: flex;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  min-height: 2.75rem;
}

.quickedit-snapshot-bar {
  flex: 1;
  padding: var(--space-05) var(--space-md);
  display: flex;
  align-items: center;
  gap: var(--space-05);
  flex-wrap: wrap;
  border-right: 1px solid var(--el-border-color-lighter);
}

.quickedit-section-label {
  font-weight: 600;
  white-space: nowrap;
}

.quickedit-snapshot-select {
  flex: 1;
  min-width: 7.5rem;
}

.quickedit-page-btn {
  padding: var(--space-xs) var(--space-05);
}

.quickedit-file-info {
  flex: 1;
  padding: var(--space-05) var(--space-md);
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}

.quickedit-file-name {
  font-weight: 600;
}

.quickedit-file-actions {
  flex-shrink: 0;
  margin-left: var(--space-md);
  display: flex;
  gap: var(--space-05);
}

.quickedit-diff-container {
  height: calc(80vh - 6.25rem);
}

.quickedit-diff-skeleton {
  height: 100%;
}

.quickedit-status-bar {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-top: var(--space-05);
  justify-content: flex-end;
}
</style>