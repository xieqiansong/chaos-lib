<script setup lang="ts">
// 存放位置字典：通用 DataTable 驱动 CRUD。
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {locationsApi} from '@/api/items'

const columns: DataTableColumn[] = [
    {field: 'Name', title: '位置名', minWidth: 160, searchable: true},
    {field: 'ParentID', title: '父级ID', width: 100},
    {field: 'SortOrder', title: '排序', width: 90, sortable: true},
    {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
]

const fields: FormField[] = [
    {field: 'Name', title: '位置名', type: 'text', span: 12, required: true},
    {field: 'ParentID', title: '父级ID(0=顶级)', type: 'number', span: 12, min: 0, defaultValue: 0},
    {field: 'SortOrder', title: '排序', type: 'number', span: 12, min: 0, defaultValue: 0},
]
</script>

<template>
    <div>
        <DataTable
            :columns="columns"
            :api="locationsApi"
            :fields="fields"
            title="存放位置"
            row-key="ID"
        />
    </div>
</template>
