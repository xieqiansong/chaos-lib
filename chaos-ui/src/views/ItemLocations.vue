<script setup lang="ts">
// 存放位置：已从独立的 item_location 表迁移到「数据字典」（dict_data, type=item_location）。
// 本页作为该字典类型的便捷管理入口，数据统一管理在 dict-data 中。
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {createDictDataApi, listDictDataAll, type DictData} from '@/api/dataDict'
import {computed, onMounted, ref} from 'vue'

const TYPE = 'item_location'
const api = createDictDataApi(TYPE)

const columns: DataTableColumn[] = [
    {field: 'Label', title: '位置名', minWidth: 160, searchable: true},
    {field: 'Value', title: '值/编码', width: 140, searchable: true},
    {field: 'ParentID', title: '父级', width: 140, formatter: parentFmt},
    {field: 'Sort', title: '排序', width: 90, sortable: true},
    {field: 'Status', title: '启用', width: 80, type: 'switch'},
    {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
]

const baseFields: FormField[] = [
    {field: 'Label', title: '位置名', type: 'text', span: 12, required: true},
    {field: 'Value', title: '值/编码', type: 'text', span: 12, required: true, placeholder: '如 书房'},
    {field: 'Sort', title: '排序', type: 'number', span: 12, min: 0, defaultValue: 0},
    {field: 'Status', title: '启用', type: 'switch', span: 12, defaultValue: true},
]

// 父级下拉与表格中的父级名称，均来自本类型下的全部数据项。
const all = ref<DictData[]>([])
const parentOptions = computed(() => [
    {label: '（顶级）', value: 0},
    ...all.value.map(d => ({label: d.Label, value: d.ID})),
])
const parentMap = computed<Record<number, string>>(() => {
    const m: Record<number, string> = {}
    for (const d of all.value) m[d.ID] = d.Label
    return m
})
function parentFmt(_r: any, v: any) {
    if (!v) return '（顶级）'
    return parentMap.value[v] ?? v
}
const fields = computed<FormField[]>(() => [
    ...baseFields,
    {field: 'ParentID', title: '父级(0=顶级)', type: 'select', span: 12, options: parentOptions.value, defaultValue: 0},
])

function refreshParents() {
    listDictDataAll(TYPE).then(r => { all.value = r })
}
onMounted(refreshParents)

function onStatus(row: DictData, next: boolean) {
    return api.setStatus(row.ID, next)
}
</script>

<template>
    <div>
        <DataTable
            :columns="columns"
            :api="api"
            :fields="fields"
            title="存放位置（数据字典）"
            row-key="ID"
            :switch-handler="onStatus"
            @loaded="refreshParents"
        />
    </div>
</template>
