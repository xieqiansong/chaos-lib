<script setup lang="ts">
// 数据字典：企业级枚举中心。先建「字典类型」（如 存放位置 / 物品分类 / 购入来源），
// 再在类型下维护「字典数据项」（可父子级）。其他业务模块统一通过 TypeCode + Value 引用，
// 后续可将个人物品等模块里的零散字典收敛到此处（迁移另议）。
import {computed, ref, watch} from 'vue'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {
    dictTypeApi,
    createDictDataApi,
    listDictDataAll,
    type DictType,
    type DictData,
} from '@/api/dataDict'

// ── 字典类型 ────────────────────────────────────────────
const typeColumns: DataTableColumn[] = [
    {field: 'Name', title: '类型名称', minWidth: 160, searchable: true},
    {field: 'Code', title: '类型编码', width: 180, searchable: true},
    {field: 'Status', title: '启用', width: 80, type: 'switch'},
    {field: 'Sort', title: '排序', width: 90, sortable: true},
    {field: 'Remark', title: '备注', minWidth: 160, searchable: true},
    {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
]

const typeFields: FormField[] = [
    {field: 'Name', title: '类型名称', type: 'text', span: 12, required: true, placeholder: '如 存放位置'},
    {field: 'Code', title: '类型编码', type: 'text', span: 12, required: true, placeholder: '如 item_location（唯一引用键）'},
    {field: 'Sort', title: '排序', type: 'number', span: 12, min: 0, defaultValue: 0},
    {field: 'Status', title: '启用', type: 'switch', span: 12, defaultValue: true},
    {field: 'Remark', title: '备注', type: 'textarea', rows: 2, placeholder: '可选说明'},
]

function onTypeStatusChange(row: DictType, next: boolean) {
    return dictTypeApi.setStatus(row.ID, next)
}

// 类型加载后回填下拉选项，并默认选中第一个。
const typeOptions = ref<{ label: string; value: string }[]>([])
function onTypesLoaded(rows: any[]) {
    typeOptions.value = (rows as DictType[]).map(t => ({label: t.Name, value: t.Code}))
    if (!selectedCode.value && typeOptions.value.length) {
        selectedCode.value = typeOptions.value[0].value
    }
}

// ── 字典数据项 ──────────────────────────────────────────
const selectedCode = ref('')

const dataColumns: DataTableColumn[] = [
    {field: 'Label', title: '显示名称', minWidth: 160, searchable: true},
    {field: 'Value', title: '值/编码', width: 160, searchable: true},
    {field: 'ParentID', title: '父级', width: 160, formatter: parentFormatter},
    {field: 'Sort', title: '排序', width: 90, sortable: true},
    {field: 'Status', title: '启用', width: 80, type: 'switch'},
    {field: 'Remark', title: '备注', minWidth: 160},
    {field: 'CreatedAt', title: '创建时间', width: 170, type: 'datetime'},
]

const dataFieldsBase: FormField[] = [
    {field: 'Label', title: '显示名称', type: 'text', span: 12, required: true, placeholder: '如 家'},
    {field: 'Value', title: '值/编码', type: 'text', span: 12, required: true, placeholder: '如 home（存业务表的取值）'},
    {field: 'Sort', title: '排序', type: 'number', span: 12, min: 0, defaultValue: 0},
    {field: 'Status', title: '启用', type: 'switch', span: 12, defaultValue: true},
    {field: 'Remark', title: '备注', type: 'textarea', rows: 2, placeholder: '可选说明'},
    {field: 'Ext', title: '扩展(JSON)', type: 'json', rows: 3, placeholder: '可选，如 {"icon":"home"}'},
]

// 当前类型下的全部数据项（用于父级下拉与名称解析）。
const allData = ref<DictData[]>([])
const parentOptions = computed(() => {
    const opts = allData.value.map(d => ({label: d.Label, value: d.ID}))
    return [{label: '（顶级）', value: 0}, ...opts]
})
const parentMap = computed<Record<number, string>>(() => {
    const m: Record<number, string> = {}
    for (const d of allData.value) m[d.ID] = d.Label
    return m
})

const dataFields = computed<FormField[]>(() => [
    ...dataFieldsBase,
    {
        field: 'ParentID',
        title: '父级(0=顶级)',
        type: 'select',
        span: 12,
        options: parentOptions.value,
        defaultValue: 0,
    },
])

const dataApi = computed(() => createDictDataApi(selectedCode.value))

function parentFormatter(_row: any, value: any): string {
    if (!value) return '（顶级）'
    return parentMap.value[value] ?? String(value)
}

async function reloadAllData() {
    allData.value = selectedCode.value ? await listDictDataAll(selectedCode.value) : []
}
watch(selectedCode, reloadAllData)

function onDataStatusChange(row: DictData, next: boolean) {
    return createDictDataApi(selectedCode.value).setStatus(row.ID, next)
}

function onTypeChange() {
    // 切换类型时刷新父级下拉，并依赖 :key 重新挂载数据表以重置查询。
    reloadAllData()
}
</script>

<template>
    <div class="dict-page">
        <el-card class="block-card">
            <template #header>
                <span class="card-title">字典类型</span>
                <span class="card-sub">先定义类型（如 存放位置），再在下方维护其数据项</span>
            </template>
            <DataTable
                :api="dictTypeApi"
                :columns="typeColumns"
                :fields="typeFields"
                title="字典类型"
                row-key="ID"
                :switch-handler="onTypeStatusChange"
                @loaded="onTypesLoaded"
            />
        </el-card>

        <el-card class="block-card">
            <template #header>
                <div class="data-header">
                    <span class="card-title">字典数据项</span>
                    <el-select
                        v-model="selectedCode"
                        placeholder="选择字典类型"
                        style="width: 220px"
                        :disabled="!typeOptions.length"
                        @change="onTypeChange"
                    >
                        <el-option
                            v-for="o in typeOptions"
                            :key="o.value"
                            :label="o.label"
                            :value="o.value"
                        />
                    </el-select>
                </div>
            </template>

            <DataTable
                v-if="selectedCode"
                :key="selectedCode"
                :api="dataApi"
                :columns="dataColumns"
                :fields="dataFields"
                title="字典数据项"
                row-key="ID"
                :switch-handler="onDataStatusChange"
            />
            <el-empty v-else description="请先在上方创建字典类型"/>
        </el-card>
    </div>
</template>

<style scoped>
.dict-page {
    display: flex;
    flex-direction: column;
    gap: var(--space-md, 16px);
}

.block-card :deep(.el-card__header) {
    display: flex;
    align-items: center;
    gap: 12px;
}

.card-title {
    font-weight: 600;
    font-size: 15px;
}

.card-sub {
    color: var(--el-text-color-secondary);
    font-size: 12px;
}

.data-header {
    display: flex;
    align-items: center;
    gap: 12px;
}
</style>
