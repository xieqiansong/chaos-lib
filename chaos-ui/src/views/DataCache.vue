<script setup lang="ts">
// 数据缓存界面：通用 DataTable（内置查看/编辑/删除 + 弹窗）驱动完整 CRUD。
// 仅声明 columns 与 fields 两份配置即可，无需任何增删改查样板。
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {dataCacheApi, COMPRESSION_OPTIONS, DATATYPE_OPTIONS} from '@/api/dataCache'
import {binarySummary} from '@/utils/binary'

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

const columns: DataTableColumn[] = [
  {field: 'Category', title: '类别', width: 120, searchable: true},
  {field: 'Key', title: 'Key', minWidth: 160, searchable: true},
  {field: 'DataType', title: '类型', width: 90, searchable: true},
  {field: 'ValueLen', title: '长度', width: 90, sortable: true},
  // 可读文本显示内容；二进制用中性标签，不铺乱码。
  {field: 'Value', title: 'Value', minWidth: 200, formatter: (_r, v) => truncate(binarySummary(String(v ?? '')), 60)},
  {field: 'ValueMd5', title: 'MD5', width: 180},
  {field: 'ExpireAt', title: '过期时间', width: 170, type: 'datetime'},
  {field: 'Compression', title: '压缩算法', width: 100},
  {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
]

// 表单字段配置（与 columns 对应，驱动 DataTable 内置的 DataFormDialog）
const fields: FormField[] = [
  {field: 'Category', title: '类别', type: 'text', span: 12, required: true, placeholder: '如 user / config / api'},
  {field: 'Key', title: 'Key', type: 'text', span: 12, required: true, placeholder: '缓存键'},
  {field: 'DataType', title: '类型', type: 'select', span: 12, options: [...DATATYPE_OPTIONS], defaultValue: 'text'},
  {field: 'Compression', title: '压缩算法', type: 'select', span: 12, options: [...COMPRESSION_OPTIONS], defaultValue: 'none'},
  // binary：查看弹窗自动按文本/中性标签展示，图片给预览并可下载；编辑时按 base64 编辑。
  {field: 'Value', title: 'Value', type: 'binary', rows: 6, placeholder: '文本内容，或 base64 编码的二进制内容（以字节原样存储）'},
  {field: 'ValueLen', title: '长度', type: 'number', span: 12, min: 0, placeholder: '值字节长度，可留空'},
  {field: 'ValueMd5', title: 'MD5', type: 'text', span: 12, placeholder: '值 MD5，可留空'},
  {field: 'ExpireAt', title: '过期时间', type: 'datetime', span: 12, placeholder: '留空表示永不过期'},
]
</script>

<template>
  <div>
    <DataTable
        :columns="columns"
        :api="dataCacheApi"
        :fields="fields"
        title="数据缓存"
        row-key="ID"
    />
  </div>
</template>