<script setup lang="ts">
import {onMounted, ref, watch} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {sendMessage} from '@/utils/api'

const props = defineProps<{
  searchText: string
}>()

interface FileLink {
  ID: number
  SourcePath: string
  TargetPath: string
  Status: boolean
  Remark: string
  Sort: number
  LinkStatus: string
}

const fileLinks = ref<FileLink[]>([])
const error = ref('')
const loading = ref(false)
const showCreateModal = ref(false)
const showEditModal = ref(false)

const newLink = ref({
  SourcePath: '',
  TargetPath: '',
  Remark: '',
  Sort: 0
})

const editLink = ref({
  ID: 0,
  Remark: '',
  Sort: 0
})

function openEditModal(link: FileLink) {
  editLink.value = {
    ID: link.ID,
    Remark: link.Remark,
    Sort: link.Sort
  }
  showEditModal.value = true
}

const linkStatusMap: Record<string, { text: string, type: string }> = {
  normal: {text: '正常', type: 'success'},
  missing: {text: '目标缺失', type: 'danger'},
  none: {text: '未启用', type: 'info'},
  invalid: {text: '无效', type: 'warning'},
  conflict: {text: '冲突', type: 'danger'}
}

async function fetchFileLinks() {
  loading.value = true
  error.value = ''
  try {
    fileLinks.value = await sendMessage('fileLinks', 'GET')
  } catch (e) {
    error.value = '获取文件连接失败'
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function createLink() {
  if (!newLink.value.SourcePath.trim() || !newLink.value.TargetPath.trim()) {
    error.value = '请填写源路径和目标路径'
    return
  }

  try {
    await sendMessage('fileLinks', 'POST', newLink.value)
    showCreateModal.value = false
    newLink.value = {SourcePath: '', TargetPath: '', Remark: '', Sort: 0}
    await fetchFileLinks()
  } catch (e) {
    error.value = '创建文件连接失败'
    console.error(e)
  }
}

async function updateEditLink() {
  try {
    await sendMessage(`fileLinks/${editLink.value.ID}`, 'PATCH', {
      Remark: editLink.value.Remark,
      Sort: editLink.value.Sort
    })
    showEditModal.value = false
    await fetchFileLinks()
  } catch (e) {
    error.value = '更新失败'
    console.error(e)
  }
}

async function toggleLinkStatus(link: FileLink, newStatus: boolean) {
  const oldStatus = !newStatus
  link.Status = newStatus
  try {
    await sendMessage(`fileLinks/${link.ID}/status`, 'PATCH', {status: newStatus})
    await fetchFileLinks()
  } catch (e) {
    link.Status = oldStatus
    error.value = '更新状态失败'
    console.error(e)
  }
}

async function deleteLink(id: number) {
  try {
    await ElMessageBox.confirm(
      '确认删除该文件连接？删除后无法恢复。',
      '警告',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch {
    return
  }

  try {
    await sendMessage(`fileLinks/${id}`, 'DELETE')
    await fetchFileLinks()
  } catch (e) {
    error.value = '删除失败'
    console.error(e)
  }
}

watch(() => props.searchText, () => {
  fetchFileLinks()
})

onMounted(() => {
  fetchFileLinks()
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">文件连接管理</span>
      <span class="text-secondary text-xs">管理本地文件符号链接</span>
      <div class="section-actions">
        <el-button size="small" type="primary" @click="showCreateModal = true">+ 创建新连接</el-button>
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

    <el-skeleton
        v-if="loading"
        :rows="3"
        animated
    />

    <div v-else-if="fileLinks.length === 0" class="empty-wrap">
      <el-empty description="暂无文件连接"/>
    </div>

    <el-table v-else :data="fileLinks" class="filelink-table">
      <el-table-column prop="SourcePath" label="源路径" min-width="200"/>
      <el-table-column prop="TargetPath" label="目标路径" min-width="200"/>
      <el-table-column prop="Remark" label="备注" min-width="120"/>
      <el-table-column prop="Sort" label="排序" width="80" sortable/>
      <el-table-column label="状态" width="100">
        <template #default="{row}">
          <el-tag :type="linkStatusMap[row.LinkStatus]?.type || 'info'">
            {{ linkStatusMap[row.LinkStatus]?.text || row.LinkStatus }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{row}">
          <el-switch
              :model-value="row.Status"
              @update:model-value="(val: boolean) => toggleLinkStatus(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{row}">
          <el-button
              type="primary"
              size="small"
              @click="openEditModal(row)">
            编辑
          </el-button>
          <el-button
              type="danger"
              size="small"
              @click="deleteLink(row.ID)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
        v-model="showCreateModal"
        title="创建文件连接"
        width="600px">
      <el-form :model="newLink" label-width="100px">
        <el-form-item label="源路径">
          <el-input
              v-model="newLink.SourcePath"
              placeholder="请输入源文件/目录路径"/>
        </el-form-item>
        <el-form-item label="目标路径">
          <el-input
              v-model="newLink.TargetPath"
              placeholder="请输入目标符号链接路径"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input
              v-model="newLink.Remark"
              type="textarea"
              :rows="2"
              placeholder="可选备注信息"/>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
              v-model="newLink.Sort"
              :min="0"
              controls-position="right"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateModal = false">取消</el-button>
        <el-button type="primary" @click="createLink">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showEditModal"
        title="编辑文件连接"
        width="600px">
      <el-form :model="editLink" label-width="100px">
        <el-form-item label="备注">
          <el-input
              v-model="editLink.Remark"
              type="textarea"
              :rows="2"
              placeholder="可选备注信息"/>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number
              v-model="editLink.Sort"
              :min="0"
              controls-position="right"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditModal = false">取消</el-button>
        <el-button type="primary" @click="updateEditLink">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.filelink-table {
  width: 100%;
}

.filelink-table :deep(.cell),
.filelink-table :deep(td.el-table__cell) {
  color: var(--term-green) !important;
}
</style>