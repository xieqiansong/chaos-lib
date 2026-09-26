<script setup lang="ts">
// 标准参考表界面：通用 DataTable（内置查看/编辑/删除 + 弹窗）驱动完整 CRUD。
// 仅声明 columns 与 fields 两份配置即可，无需任何增删改查样板。
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {standardDataApi, type StandardData} from '@/api/standardData'

const columns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 160, searchable: true},
  {field: 'Code', title: '编码', width: 140, searchable: true},
  {field: 'Category', title: '分类', width: 120},
  {field: 'Quantity', title: '数量', width: 90, sortable: true},
  {field: 'Price', title: '金额', width: 100},
  {field: 'Enabled', title: '启用', width: 80, type: 'switch'},
  {field: 'Config', title: '配置(JSON)', minWidth: 160},
  {field: 'EffectiveAt', title: '生效时间', width: 170, type: 'datetime'},
  {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
  {field: 'UpdatedAt', title: '更新时间', width: 170, type: 'datetime'},
]

// 表单字段配置（与 columns 对应，驱动 DataTable 内置的 DataFormDialog）
const fields: FormField[] = [
  {field: 'Name', title: '名称', type: 'text', span: 12, required: true, placeholder: '请输入名称'},
  {field: 'Code', title: '编码', type: 'text', span: 12, required: true, placeholder: '请输入编码'},
  {field: 'Category', title: '分类', type: 'text', span: 12, placeholder: '可选分类'},
  {field: 'Sort', title: '排序', type: 'number', span: 12, min: 0},
  {field: 'Quantity', title: '数量', type: 'number', span: 12, min: 0},
  {field: 'Price', title: '金额', type: 'number', span: 12, min: 0, precision: 2, step: 0.01},
  {field: 'Enabled', title: '启用', type: 'switch', span: 12},
  {field: 'EffectiveAt', title: '生效时间', type: 'datetime', span: 12},
  {field: 'Description', title: '描述', type: 'textarea', rows: 2, placeholder: '可选描述'},
  {field: 'Config', title: '配置(JSON)', type: 'json', rows: 3, placeholder: '如 {"k":"v"}'},
]

// 状态切换（type:'switch' 列）由本页作为「标准 CRUD 之外的自定义逻辑」注入 DataTable：
// 通过 switch-handler 回调调用 standardDataApi.setStatus，DataTable 负责刷新与提示。
// 时间列（type:'datetime'）由 DataTable 按标准格式序列化显示，无需页级代码。
function onEnabledChange(row: StandardData, next: boolean) {
  return standardDataApi.setStatus(row.ID, next)
}
</script>

<template>
  <div>
    <DataTable
        :columns="columns"
        :api="standardDataApi"
        :fields="fields"
        title="标准数据"
        row-key="ID"
        :switch-handler="onEnabledChange"
    />
  </div>
</template>
