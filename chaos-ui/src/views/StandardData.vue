<script setup lang="ts">
// 标准参考表界面：通用 DataTable（内置查看/编辑/删除 + 弹窗）驱动完整 CRUD。
// 仅声明 columns 与 fields 两份配置即可，无需任何增删改查样板。
import {ref} from 'vue'
import {ElMessage} from 'element-plus'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn} from '@/components/dataTable/types'
import type {FormField} from '@/components/DataFormDialog.vue'
import {standardDataApi, type StandardData} from '@/api/standardData'

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
]

// 表单字段配置（与 columns 对应，驱动 DataTable 内置的 DataFormDialog）
const fields: FormField[] = [
  {field: 'Name', title: '名称', type: 'text', required: true, placeholder: '请输入名称'},
  {field: 'Code', title: '编码', type: 'text', required: true, placeholder: '请输入编码'},
  {field: 'Category', title: '分类', type: 'text', placeholder: '可选分类'},
  {field: 'Description', title: '描述', type: 'textarea', rows: 2, placeholder: '可选描述'},
  {field: 'Quantity', title: '数量', type: 'number', min: 0},
  {field: 'Price', title: '金额', type: 'number', min: 0, precision: 2, step: 0.01},
  {field: 'Enabled', title: '启用', type: 'switch'},
  {field: 'Config', title: '配置(JSON)', type: 'json', rows: 3, placeholder: '如 {"k":"v"}'},
  {field: 'EffectiveAt', title: '生效时间', type: 'datetime'},
  {field: 'Sort', title: '排序', type: 'number', min: 0},
]

// 启停开关对应后端 setStatus；行内即时反馈由 DataTable 刷新保证，此处仅触发调用
const tableRef = ref<InstanceType<typeof DataTable>>()
function setStatus(row: StandardData, next: boolean) {
  standardDataApi.setStatus(row.ID, next)
      .then(() => ElMessage.success('状态已更新'))
      .catch((e: any) => ElMessage.error(e?.message || '更新状态失败'))
      .finally(() => tableRef.value?.refresh())
}
</script>

<template>
  <div>
    <DataTable
        ref="tableRef"
        :columns="columns"
        :api="standardDataApi"
        :fields="fields"
        title="标准数据"
        row-key="ID"
    >
      <template #Enabled="{ row }">
        <el-switch
            :model-value="row.Enabled"
            @update:model-value="(val: boolean) => setStatus(row, val)"
        />
      </template>
    </DataTable>
  </div>
</template>
