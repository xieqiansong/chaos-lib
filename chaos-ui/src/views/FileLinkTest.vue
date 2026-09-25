<script setup lang="ts">
// 临时测试界面：用通用 DataTable 组件复刻「文件连接」能力，用于验证配置驱动表格方案。
// 故意不修改原 FileLink.vue —— 本文件为新增的试点，可随时删除。
import {ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {sendMessage} from '@/utils/api'
import DataTable from '@/components/DataTable.vue'
import type {DataTableApiParams, DataTableColumn} from '@/components/dataTable/types'

// 兼容 App.vue 向动态视图透传的 search-text（本试点用 DataTable 自带搜索，故未使用）
defineProps<{ searchText?: string }>()

interface FileLink {
  ID: number
  SourcePath: string
  TargetPath: string
  Status: boolean
  Remark: string
  Sort: number
  LinkStatus: string
}

const showCreateModal = ref(false)
const showEditModal = ref(false)
const newLink = ref({SourcePath: '', TargetPath: '', Remark: '', Sort: 0})
const editLink = ref({ID: 0, Remark: '', Sort: 0})

const linkStatusMap: Record<string, { text: string; type: string }> = {
  normal: {text: '正常', type: 'success'},
  missing: {text: '目标缺失', type: 'danger'},
  none: {text: '未启用', type: 'info'},
  invalid: {text: '无效', type: 'warning'},
  conflict: {text: '冲突', type: 'danger'},
}

// 1) 列配置：searchable 列自动生成搜索栏；操作列用 type:'actions' + #actions 插槽
const columns: DataTableColumn[] = [
  {field: 'SourcePath', title: '源路径', minWidth: 200, searchable: true, search: {placeholder: '源路径关键词'}},
  {field: 'TargetPath', title: '目标路径', minWidth: 200, searchable: true},
  {field: 'Remark', title: '备注', minWidth: 120, searchable: true},
  {field: 'Sort', title: '排序', width: 80, sortable: true},
  {field: 'LinkStatus', title: '状态', width: 100},
  {field: 'Status', title: '启用', width: 80},
  {field: '__actions', title: '操作', width: 160, type: 'actions', fixed: 'right'},
]

// 2) 服务端取数适配器：把 DataTable 的 { page, pageSize, search } 翻译成后端 fileLinks 约定
async function fetchApi(params: DataTableApiParams) {
  const q = Object.values(params.search).filter(Boolean).join(' ').trim()
  const query: Record<string, any> = {page: params.page, size: params.pageSize}
  if (q) query.search = q
  if (params.sort?.order) {
    query.sort = params.sort.field
    query.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
  }
  const res = await sendMessage('fileLinks', 'GET', query)
  return {rows: res?.items ?? [], total: res?.total ?? 0}
}

// ---- 以下 CRUD 逻辑与原 FileLink 一致，仅作为试点演示 ----
function openEditModal(link: FileLink) {
  editLink.value = {ID: link.ID, Remark: link.Remark, Sort: link.Sort}
  showEditModal.value = true
}

async function createLink() {
  if (!newLink.value.SourcePath.trim() || !newLink.value.TargetPath.trim()) {
    ElMessage.error('请填写源路径和目标路径')
    return
  }
  try {
    await sendMessage('fileLinks', 'POST', newLink.value)
    showCreateModal.value = false
    newLink.value = {SourcePath: '', TargetPath: '', Remark: '', Sort: 0}
    ElMessage.success('创建成功')
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  }
}

async function updateEditLink() {
  try {
    await sendMessage(`fileLinks/${editLink.value.ID}`, 'PATCH', {
      Remark: editLink.value.Remark,
      Sort: editLink.value.Sort,
    })
    showEditModal.value = false
    ElMessage.success('保存成功')
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '更新失败')
  }
}

async function toggleLinkStatus(link: FileLink, next: boolean) {
  const prev = link.Status
  link.Status = next
  try {
    await sendMessage(`fileLinks/${link.ID}/status`, 'PATCH', {status: next})
    ElMessage.success('状态已更新')
    refresh()
  } catch (e: any) {
    link.Status = prev
    ElMessage.error(e?.message || '更新状态失败')
  }
}

async function deleteLink(id: number) {
  try {
    await ElMessageBox.confirm('确认删除该文件连接？删除后无法恢复。', '警告', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await sendMessage(`fileLinks/${id}`, 'DELETE')
    ElMessage.success('删除成功')
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

// DataTable 暴露的 refresh（通过 ref 获取）
const tableRef = ref<InstanceType<typeof DataTable>>()

function refresh() {
  tableRef.value?.refresh()
}
</script>

<template>
  <div>
    <DataTable ref="tableRef" :columns="columns" :api="fetchApi" row-key="ID">
      <template #toolbar>
        <el-button size="small" type="primary" @click="showCreateModal = true">+ 创建新连接</el-button>
      </template>

      <template #LinkStatus="{ row }">
        <el-tag :type="(linkStatusMap[row.LinkStatus]?.type as any) || 'info'">
          {{ linkStatusMap[row.LinkStatus]?.text || row.LinkStatus }}
        </el-tag>
      </template>

      <template #Status="{ row }">
        <el-switch
            :model-value="row.Status"
            @update:model-value="(val: boolean) => toggleLinkStatus(row, val)"
        />
      </template>

      <template #actions="{ row }">
        <el-button type="primary" size="small" text @click="openEditModal(row)">编辑</el-button>
        <el-button type="danger" size="small" text @click="deleteLink(row.ID)">删除</el-button>
      </template>
    </DataTable>

    <el-dialog v-model="showCreateModal" title="创建文件连接" width="37.5rem">
      <el-form :model="newLink" label-width="6.25rem">
        <el-form-item label="源路径">
          <el-input v-model="newLink.SourcePath" placeholder="请输入源文件/目录路径"/>
        </el-form-item>
        <el-form-item label="目标路径">
          <el-input v-model="newLink.TargetPath" placeholder="请输入目标符号链接路径"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="newLink.Remark" type="textarea" :rows="2" placeholder="可选备注信息"/>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="newLink.Sort" :min="0" controls-position="right"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateModal = false">取消</el-button>
        <el-button type="primary" @click="createLink">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditModal" title="编辑文件连接" width="37.5rem">
      <el-form :model="editLink" label-width="6.25rem">
        <el-form-item label="备注">
          <el-input v-model="editLink.Remark" type="textarea" :rows="2" placeholder="可选备注信息"/>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="editLink.Sort" :min="0" controls-position="right"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditModal = false">取消</el-button>
        <el-button type="primary" @click="updateEditLink">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
