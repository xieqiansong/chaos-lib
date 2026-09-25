<script setup lang="ts">
// 标准参考表界面：用通用 DataTable + 每资源 api（src/api/standardData.ts）实现完整 CRUD。
// 作为「简单表只定义 API 前缀即可一键化处理」的基准范式：
//   - 列表/搜索/排序：DataTable + standardDataApi.fetch（无需 fetchApi 样板）
//   - 增/改/删/状态：standardDataApi.create/update/remove/setStatus
import {reactive, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn} from '@/components/dataTable/types'
import {standardDataApi, type StandardData} from '@/api/standardData'

const tableRef = ref<InstanceType<typeof DataTable>>()

const columns: DataTableColumn[] = [
  {field: 'ID', title: 'ID', width: 70, sortable: true},
  {field: 'Name', title: '名称', minWidth: 160, searchable: true},
  {field: 'Code', title: '编码', width: 140, searchable: true},
  {field: 'Category', title: '分类', width: 120},
  {field: 'Quantity', title: '数量', width: 90, sortable: true},
  {field: 'Price', title: '金额', width: 100},
  {field: 'Enabled', title: '启用', width: 80},
  {field: 'Config', title: '配置(JSON)', minWidth: 160},
  {field: 'EffectiveAt', title: '生效时间', width: 170},
  {field: 'Sort', title: '排序', width: 80, sortable: true},
  {field: 'CreatedAt', title: '创建时间', width: 170},
  {field: 'UpdatedAt', title: '更新时间', width: 170},
  {field: '__actions', title: '操作', width: 160, type: 'actions', fixed: 'right'},
]

// ---- 表单（新建 / 编辑共用） ----
const showDialog = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingId = ref<number | null>(null)
const saving = ref(false)

function emptyForm(): Partial<StandardData> {
  return {
    Name: '',
    Code: '',
    Description: '',
    Category: '',
    Quantity: 0,
    Price: 0,
    Enabled: true,
    Config: '',
    EffectiveAt: null,
    Sort: 0,
  }
}

const form = reactive<Partial<StandardData>>(emptyForm())

function openCreate() {
  dialogMode.value = 'create'
  editingId.value = null
  Object.assign(form, emptyForm())
  showDialog.value = true
}

function openEdit(row: StandardData) {
  dialogMode.value = 'edit'
  editingId.value = row.ID
  Object.assign(form, {
    Name: row.Name,
    Code: row.Code,
    Description: row.Description,
    Category: row.Category,
    Quantity: row.Quantity,
    Price: row.Price,
    Enabled: row.Enabled,
    Config: row.Config,
    EffectiveAt: row.EffectiveAt,
    Sort: row.Sort,
  })
  showDialog.value = true
}

async function save() {
  if (!form.Name || !form.Code) {
    ElMessage.error('名称与编码不能为空')
    return
  }
  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await standardDataApi.create({...form})
      ElMessage.success('创建成功')
    } else if (editingId.value != null) {
      await standardDataApi.update(editingId.value, {...form})
      ElMessage.success('更新成功')
    }
    showDialog.value = false
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function setStatus(row: StandardData, next: boolean) {
  try {
    await standardDataApi.setStatus(row.ID, next)
    ElMessage.success('状态已更新')
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '更新状态失败')
  }
}

async function remove(row: StandardData) {
  try {
    await ElMessageBox.confirm(`确认删除「${row.Name}」？删除后可在库中恢复（逻辑删除）。`, '警告', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await standardDataApi.remove(row.ID)
    ElMessage.success('删除成功')
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}
</script>

<template>
  <div>
    <DataTable ref="tableRef" :columns="columns" :api="standardDataApi.fetch" row-key="ID">
      <template #toolbar>
        <el-button size="small" type="primary" @click="openCreate">+ 新建标准数据</el-button>
      </template>

      <template #Enabled="{ row }">
        <el-switch
            :model-value="row.Enabled"
            @update:model-value="(val: boolean) => setStatus(row, val)"
        />
      </template>

      <template #actions="{ row }">
        <el-button type="primary" size="small" text @click="openEdit(row)">编辑</el-button>
        <el-button type="danger" size="small" text @click="remove(row)">删除</el-button>
      </template>
    </DataTable>

    <el-dialog
        v-model="showDialog"
        :title="dialogMode === 'create' ? '新建标准数据' : '编辑标准数据'"
        width="37.5rem"
    >
      <el-form :model="form" label-width="6.25rem">
        <el-form-item label="名称" required>
          <el-input v-model="form.Name" placeholder="请输入名称"/>
        </el-form-item>
        <el-form-item label="编码" required>
          <el-input v-model="form.Code" placeholder="请输入编码"/>
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.Category" placeholder="可选分类"/>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.Description" type="textarea" :rows="2" placeholder="可选描述"/>
        </el-form-item>
        <el-form-item label="数量">
          <el-input-number v-model="form.Quantity" :min="0" controls-position="right"/>
        </el-form-item>
        <el-form-item label="金额">
          <el-input-number v-model="form.Price" :min="0" :precision="2" :step="0.01" controls-position="right"/>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.Enabled"/>
        </el-form-item>
        <el-form-item label="配置(JSON)">
          <el-input v-model="form.Config" type="textarea" :rows="3" placeholder='如 {"k":"v"}'/>
        </el-form-item>
        <el-form-item label="生效时间">
          <el-date-picker
              v-model="form.EffectiveAt"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="可选"
              style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.Sort" :min="0" controls-position="right"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
