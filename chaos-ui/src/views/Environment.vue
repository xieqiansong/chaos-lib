<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {ElMessage} from 'element-plus'
import {API_BASE} from '@/utils/api'

interface EnvResponse {
  Meta: { SavedAt: string; Hostname: string; Username: string }
  System: Record<string, string>
  User: Record<string, string>
  SnapshotId: number
  SnapshotTime: string
  Warnings: string[]
}

const data = ref<EnvResponse | null>(null)
const loading = ref(false)

const activeTab = ref<'system' | 'user'>('user')
const filterText = ref('')

const filteredVariables = computed(() => {
  const section = activeTab.value === 'system' ? data.value?.System : data.value?.User
  if (!section) return []
  const q = filterText.value.toLowerCase()
  return Object.entries(section)
      .filter(([k, v]) => !q || k.toLowerCase().includes(q) || v.toLowerCase().includes(q))
      .sort(([a], [b]) => a.localeCompare(b))
})

async function fetchEnv() {
  loading.value = true
  try {
    const res = await fetch(API_BASE + '/envVariables')
    if (!res.ok) throw new Error('获取环境变量失败')
    data.value = await res.json()
  } catch (e: any) {
    ElMessage.error(e.message || '获取环境变量失败')
  } finally {
    loading.value = false
  }
}

async function syncEnv() {
  loading.value = true
  try {
    const res = await fetch(API_BASE + '/envVariables/sync', {method: 'POST'})
    const result = await res.json()
    if (!res.ok) throw new Error(result.error || '同步失败')
    ElMessage.success('同步成功')
    await fetchEnv()
  } catch (e: any) {
    ElMessage.error(e.message || '同步失败')
  } finally {
    loading.value = false
  }
}

const editingKey = ref('')
const editValue = ref('')
const editSaving = ref(false)
const editArrayItems = ref<string[]>([])

function startEdit(key: string, value: string) {
  editingKey.value = key
  editValue.value = value
  editArrayItems.value = value.split(';')
  editMode.value = 'array'
}

function cancelEdit() {
  editingKey.value = ''
  editValue.value = ''
  editArrayItems.value = []
  editMode.value = 'array'
}

function switchToArrayMode() {
  editArrayItems.value = editValue.value.split(';')
  editMode.value = 'array'
}

function switchToTextMode() {
  editValue.value = editArrayItems.value.join(';')
  editMode.value = 'text'
}

function addArrayItem() {
  editArrayItems.value.push('')
}

function removeArrayItem(index: number) {
  editArrayItems.value.splice(index, 1)
}

const draggingArrayIndex = ref(-1)

function onArrayDragStart(index: number, e: DragEvent) {
  draggingArrayIndex.value = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
  }
}

function onArrayDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
}

function onArrayDrop(index: number) {
  const from = draggingArrayIndex.value
  if (from < 0 || from === index) return
  const items = [...editArrayItems.value]
  const [moved] = items.splice(from, 1)
  items.splice(index, 0, moved)
  editArrayItems.value = items
}

function onArrayDragEnd() {
  draggingArrayIndex.value = -1
}

function onEditModeChange(mode: 'text' | 'array') {
  if (mode === 'array') switchToArrayMode()
  else switchToTextMode()
}

async function saveEdit() {
  if (!data.value) return
  editSaving.value = true
  const scope = activeTab.value
  const value = editMode.value === 'array' ? editArrayItems.value.join(';') : editValue.value
  const payload = {[scope]: {set: {[editingKey.value]: value}}}
  try {
    const res = await fetch(API_BASE + '/envVariables', {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    })
    const result = await res.json()
    if (!res.ok) throw new Error(result.error || '保存失败')
    if (result.warnings?.length) ElMessage.warning(result.warnings.join('; '))
    else ElMessage.success('保存成功')
    editingKey.value = ''
    await fetchEnv()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    editSaving.value = false
  }
}

async function deleteVar(key: string) {
  if (!data.value) return
  const scope = activeTab.value
  const payload = {[scope]: {unset: [key]}}
  try {
    const res = await fetch(API_BASE + '/envVariables', {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    })
    const result = await res.json()
    if (!res.ok) throw new Error(result.error || '删除失败')
    ElMessage.success(`已删除 ${key}`)
    await fetchEnv()
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

const showAddDialog = ref(false)
const newKey = ref('')
const newValue = ref('')
const addSaving = ref(false)

async function addVariable() {
  if (!newKey.value.trim()) return
  addSaving.value = true
  const scope = activeTab.value
  const payload = {[scope]: {set: {[newKey.value.trim()]: newValue.value}}}
  try {
    const res = await fetch(API_BASE + '/envVariables', {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    })
    const result = await res.json()
    if (!res.ok) throw new Error(result.error || '添加失败')
    ElMessage.success(`已添加 ${newKey.value}`)
    showAddDialog.value = false
    newKey.value = ''
    newValue.value = ''
    await fetchEnv()
  } catch (e: any) {
    ElMessage.error(e.message || '添加失败')
  } finally {
    addSaving.value = false
  }
}

const editMode = ref<'text' | 'array'>('array')

onMounted(() => {
  fetchEnv()
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">环境变量管理</span>
      <span v-if="data" class="text-secondary text-xs">
        {{ data.Meta?.Hostname || '本机' }}
      </span>
      <div class="section-actions">
        <el-button size="small" @click="fetchEnv" :loading="loading" plain>刷新</el-button>
        <el-button size="small" @click="syncEnv" :loading="loading">同步到快照</el-button>
      </div>
    </div>

    <el-alert
        v-for="(w, i) in data?.Warnings ?? []"
        :key="i"
        type="warning"
        :message="w"
        show-icon
        class="mb-sm"
        @close="data!.Warnings = data!.Warnings.filter((_, j) => j !== i)"
    />

    <el-skeleton v-if="loading && !data" :rows="8" animated/>

    <template v-if="data">
      <el-tabs v-model="activeTab" class="env-tabs">
        <el-tab-pane label="用户变量" name="user"/>
        <el-tab-pane name="system">
          <template #label>
            系统变量
          </template>
        </el-tab-pane>
      </el-tabs>

      <div class="env-filter-bar">
        <el-input
            v-model="filterText"
            placeholder="搜索变量名或值..."
            size="small"
            clearable
            class="env-filter-input"
        />
        <el-button
            size="small"
            type="primary"
            @click="showAddDialog = true"
        >
          添加变量
        </el-button>
      </div>

      <el-table
          :data="filteredVariables"
          size="small"
          stripe
          max-height="50vh"
          class="env-table"
      >
        <el-table-column prop="0" label="变量名" min-width="180">
          <template #default="{ row }">
            <code>{{ row[0] }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="1" label="值" min-width="300">
          <template #default="{ row }">
            <template v-if="editingKey === row[0]">
              <div class="edit-container">
                <div class="edit-mode-toggle">
                  <el-radio-group v-model="editMode" size="small" @change="onEditModeChange">
                    <el-radio-button value="text">文本</el-radio-button>
                    <el-radio-button value="array">数组</el-radio-button>
                  </el-radio-group>
                </div>
                <template v-if="editMode === 'text'">
                  <div class="edit-row">
                    <el-input v-model="editValue" size="small"/>
                  </div>
                </template>
                <template v-else>
                  <div class="array-editor">
                    <div
                        v-for="(item, idx) in editArrayItems"
                        :key="idx"
                        class="array-item-row"
                        :class="{ 'array-item-dragging': draggingArrayIndex === idx }"
                        draggable="true"
                        @dragstart="onArrayDragStart(idx, $event)"
                        @dragover="onArrayDragOver($event)"
                        @drop="onArrayDrop(idx)"
                        @dragend="onArrayDragEnd"
                    >
                      <span class="array-drag-handle">
                        <el-icon><svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><circle cx="9" cy="5" r="1.5"/><circle cx="15" cy="5"
                                                                                                                                            r="1.5"/><circle
                            cx="9" cy="12" r="1.5"/><circle cx="15" cy="12" r="1.5"/><circle cx="9" cy="19" r="1.5"/><circle cx="15" cy="19"
                                                                                                                             r="1.5"/></svg></el-icon>
                      </span>
                      <el-input v-model="editArrayItems[idx]" size="small"/>
                      <el-button size="small" type="danger" @click="removeArrayItem(idx)" circle text>
                        <el-icon>
                          <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor">
                            <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                          </svg>
                        </el-icon>
                      </el-button>
                    </div>
                    <el-button size="small" @click="addArrayItem" class="array-add-btn">+ 添加条目</el-button>
                  </div>
                </template>
                <div class="edit-actions">
                  <el-button size="small" type="primary" @click="saveEdit" :loading="editSaving">保存</el-button>
                  <el-button size="small" @click="cancelEdit">取消</el-button>
                </div>
              </div>
            </template>
            <span v-else class="env-value">{{ row[1] }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="startEdit(row[0], row[1])" text>编辑</el-button>
            <el-button size="small" type="danger" @click="deleteVar(row[0])" text>删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <el-dialog v-model="showAddDialog" title="添加环境变量" width="30rem">
      <el-form label-position="top">
        <el-form-item label="变量名">
          <el-input v-model="newKey" placeholder="例如: JAVA_HOME"/>
        </el-form-item>
        <el-form-item label="变量值">
          <el-input v-model="newValue" placeholder="例如: C:\Program Files\Java\jdk-17"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="addVariable" :loading="addSaving">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.env-tabs {
  margin-bottom: var(--space-sm);
}

.env-filter-bar {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
  align-items: center;
}

.env-filter-input {
  flex: 1;
  max-width: 18rem;
}

.env-table {
  width: 100%;
}

.env-value {
  word-break: break-all;
}

.edit-container {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.edit-mode-toggle {
  display: flex;
  align-items: center;
}

.edit-actions {
  display: flex;
  gap: var(--space-05);
}

.array-editor {
  display: flex;
  flex-direction: column;
  gap: var(--space-05);
}

.array-item-row {
  display: flex;
  gap: var(--space-05);
  align-items: center;
  cursor: grab;
}

.array-item-row:active {
  cursor: grabbing;
}

.array-item-dragging {
  opacity: 0.4;
}

.array-drag-handle {
  display: flex;
  align-items: center;
  color: var(--el-text-color-placeholder);
  flex-shrink: 0;
}

.array-add-btn {
  align-self: flex-start;
}

.edit-row {
  display: flex;
  gap: var(--space-05);
  align-items: center;
}
</style>