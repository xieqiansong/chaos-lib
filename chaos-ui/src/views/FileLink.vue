<script setup lang="ts">
// 文件连接界面：参考标准数据，由通用 DataTable（内置查看/编辑/删除 + 弹窗）驱动完整 CRUD。
// 仅声明 columns 与 fields 两份配置即可，无需任何增删改查样板。
//
// 本资源相对标准基线有两个「自定义」点：
//   1. 状态（LinkStatus）由后端在返回时按文件系统实时计算，不落库 —— 前端只负责渲染。
//   2. 启用开关有副作用（建/删联接点）且带校验，走自定义 /:id/status 接口，
//      由 switch-handler 回调注入（与标准数据的 status 同一套范式）。
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn} from '@/components/dataTable/types'
import type {FormField} from '@/components/DataFormDialog.vue'
import {fileLinkApi, type FileLink} from '@/api/fileLink'

// 兼容 App.vue 向动态视图透传的 search-text（本页用 DataTable 自带搜索，故未使用）
defineProps<{ searchText?: string }>()

const columns: DataTableColumn[] = [
  {field: 'SourcePath', title: '源路径', minWidth: 200, searchable: true},
  {field: 'TargetPath', title: '目标路径', minWidth: 200, searchable: true},
  {field: 'Remark', title: '备注', minWidth: 120, searchable: true},
  {field: 'Sort', title: '排序', width: 80, sortable: true},
  {field: 'LinkStatus', title: '状态', width: 100},
  {field: 'Status', title: '启用', width: 80, type: 'switch'},
]

// 表单字段配置（与 columns 对应，驱动 DataTable 内置的 DataFormDialog）
const fields: FormField[] = [
  {field: 'SourcePath', title: '源路径', type: 'text', required: true, placeholder: '请输入源文件/目录路径'},
  {field: 'TargetPath', title: '目标路径', type: 'text', required: true, placeholder: '请输入目标符号链接路径'},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, placeholder: '可选备注信息'},
  {field: 'Sort', title: '排序', type: 'number', span: 12, min: 0},
]

// 后端默认按 id 排序；本页按业务语义默认使用 Sort 字段升序
const defaultSort = {field: 'Sort', order: 'ascending'} as const

const linkStatusMap: Record<string, { text: string; type: string }> = {
  normal: {text: '正常', type: 'success'},
  missing: {text: '目标缺失', type: 'danger'},
  none: {text: '未启用', type: 'info'},
  invalid: {text: '无效', type: 'warning'},
  conflict: {text: '冲突', type: 'danger'},
}

// 状态切换：交给 fileLinkApi 的自定义 setStatus（DataTable 负责刷新与提示）
function onStatusChange(row: FileLink, next: boolean) {
  return fileLinkApi.setStatus(row.ID, next)
}
</script>

<template>
  <div class="filelink-view">
    <DataTable
        :columns="columns"
        :api="fileLinkApi"
        :fields="fields"
        title="文件连接"
        row-key="ID"
        :default-sort="defaultSort"
        :switch-handler="onStatusChange"
    >
      <template #LinkStatus="{ row }">
        <el-tag :type="(linkStatusMap[row.LinkStatus]?.type as any) || 'info'">
          {{ linkStatusMap[row.LinkStatus]?.text || row.LinkStatus }}
        </el-tag>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
.filelink-view :deep(.datatable .cell),
.filelink-view :deep(.datatable td.el-table__cell) {
  color: var(--term-green) !important;
}
</style>
