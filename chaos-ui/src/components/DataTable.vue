<script setup lang="ts">
// 配置驱动的通用表格 + 增删改查组件。
// 由 columns 派生表格列；searchable 列自动生成搜索栏；支持服务端(api)/本地(data)两种分页模式。
// 自定义单元格优先用具名插槽 #field，其次 formatter，否则纯文本。
// 操作列：传入完整 RestApi（含 create/update/remove/setStatus）+ fields 时，自动渲染「查看/编辑/删除」并内置弹窗；
//         也可自行用 columns 的 { type: 'actions' } + 具名插槽 #actions 自定义（保持向后兼容）。
import {computed, onMounted, reactive, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {useDataTable} from '@/composables/useDataTable'
import {type RestApi} from '@/composables/useRestApi'
import type {DataTableApiParams, DataTableApiResult, DataTableColumn} from '@/components/dataTable/types'
import DataFormDialog, {type FormField} from '@/components/DataFormDialog.vue'

const props = withDefaults(
    defineProps<{
      columns: DataTableColumn[]
      /** 本地模式：直接传入数组 */
      data?: any[]
      /** 服务端模式：传入取数函数，或完整的 RestApi 对象（同时启用内置增删改查） */
      api?: RestApi<any> | ((params: DataTableApiParams) => Promise<DataTableApiResult>)
      rowKey?: string
      pageSize?: number
      pageSizeOptions?: number[]
      selection?: boolean
      treeProps?: { children: string; hasChildren?: string }
      /** 资源名，用于内置弹窗/按钮文案，如「标准数据」 */
      title?: string
      /** 表单字段配置；与 api 为 RestApi 时启用内置增删改查弹窗 */
      fields?: FormField[]
      /** 删除确认时用于展示的记录名称字段，默认 Name */
      nameField?: string
      stripe?: boolean
      border?: boolean
    }>(),
    {
      data: undefined,
      api: undefined,
      rowKey: 'ID',
      pageSize: 10,
      pageSizeOptions: () => [10, 20, 50, 100],
      selection: false,
      stripe: true,
      border: true,
      title: '',
      fields: undefined,
      nameField: 'Name',
    },
)

// api 为完整 RestApi 对象时启用内置 CRUD；仅传取数函数则仅做表格。
const isCrud = computed(() => !!props.api && typeof props.api !== 'function')
const crudApi = computed(() => (isCrud.value ? (props.api as RestApi<any>) : undefined))
const fetchFn = computed(() =>
    typeof props.api === 'function' ? props.api : props.api ? props.api.fetch : undefined,
)
const crudMode = computed(() => isCrud.value && !!props.fields)

const {
  page,
  pageSize,
  total,
  loading,
  searchState,
  searchableColumns,
  tableData,
  getData,
  handleSearch,
  handleReset,
  handlePageChange,
  handleSizeChange,
  handleSortChange,
  refresh,
} = useDataTable({
  columns: props.columns,
  data: props.data,
  api: props.api
    ? (p: DataTableApiParams) =>
        fetchFn.value ? fetchFn.value(p) : Promise.resolve({rows: [], total: 0})
    : undefined,
  pageSize: props.pageSize,
  pageSizeOptions: props.pageSizeOptions,
})

onMounted(() => getData())

// 内置 CRUD 模式且未显式声明 actions 列时，自动追加操作列
const finalColumns = computed<DataTableColumn[]>(() => {
  if (!crudMode.value) return props.columns
  if (props.columns.some((c) => c.type === 'actions')) return props.columns
  return [
    ...props.columns,
    {field: '__actions', title: '操作', width: 160, type: 'actions', fixed: 'right'} as DataTableColumn,
  ]
})

// ---- 内置 查看 / 编辑 / 删除 ----
const dialogMode = ref<'create' | 'edit'>('create')
const showDialog = ref(false)
const showView = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const form = reactive<Record<string, any>>({})

function emptyForm(): Record<string, any> {
  const f: Record<string, any> = {}
  for (const field of props.fields ?? []) {
    if (field.type === 'switch') f[field.field] = true
    else if (field.type === 'number') f[field.field] = 0
    else f[field.field] = field.type === 'datetime' ? null : ''
  }
  return f
}

function fillForm(row: any) {
  for (const field of props.fields ?? []) {
    form[field.field] = row[field.field]
  }
}

function openCreate() {
  dialogMode.value = 'create'
  editingId.value = null
  Object.assign(form, emptyForm())
  showDialog.value = true
}

function openEdit(row: any) {
  dialogMode.value = 'edit'
  editingId.value = row[props.rowKey]
  fillForm(row)
  showDialog.value = true
}

function openView(row: any) {
  fillForm(row)
  showView.value = true
}

async function save() {
  if (!crudApi.value) return
  for (const field of props.fields ?? []) {
    if (field.required && !form[field.field]) {
      ElMessage.error(`${field.title}不能为空`)
      return
    }
  }
  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await crudApi.value.create({...form})
      ElMessage.success('创建成功')
    } else if (editingId.value != null) {
      await crudApi.value.update(editingId.value, {...form})
      ElMessage.success('更新成功')
    }
    showDialog.value = false
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  if (!crudApi.value) return
  const name = row[props.nameField] ?? row[props.rowKey]
  try {
    await ElMessageBox.confirm(
        `确认删除「${name}」？删除后可在库中恢复（逻辑删除）。`,
        '警告',
        {confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning'},
    )
  } catch {
    return
  }
  try {
    await crudApi.value.remove(row[props.rowKey])
    ElMessage.success('删除成功')
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

defineExpose({refresh, getData})
</script>

<template>
  <section class="section-toolbar datatable-toolbar">
    <div v-if="$slots.toolbar || crudMode" class="toolbar-left">
      <slot name="toolbar"/>
      <el-button v-if="crudMode" size="small" type="primary" @click="openCreate">+ 新建{{ title }}</el-button>
    </div>
    <div v-if="searchableColumns.length" class="toolbar-search">
      <el-form :inline="true" @submit.prevent>
        <el-form-item v-for="col in searchableColumns" :key="col.field" :label="col.search?.label ?? col.title">
          <el-select v-if="col.search?.type === 'select'" v-model="searchState[col.field]" :placeholder="col.search?.placeholder ?? '请选择'" clearable
                     style="width: 160px">
            <el-option v-for="o in (col.search?.options ?? [])" :key="o.value" :label="o.label" :value="o.value"/>
          </el-select>
          <el-input v-else v-model="searchState[col.field]" :placeholder="col.search?.placeholder ?? '请输入'" clearable style="width: 200px"
                    @keyup.enter="handleSearch" size="small"/>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch" size="small">查询</el-button>
          <el-button text @click="handleReset" size="small">重置</el-button>
        </el-form-item>
      </el-form>
    </div>
  </section>

  <el-table v-loading="loading" :data="tableData" :row-key="rowKey" :tree-props="treeProps" :stripe="stripe" :border="border" class="datatable"
            @sort-change="handleSortChange" size="small">
    <el-table-column v-if="selection" type="selection" width="48"/>

    <el-table-column v-for="col in finalColumns"
                     :key="col.field" :prop="col.field" :label="col.title" :width="col.width"
                     :min-width="col.minWidth" :align="col.align" :fixed="col.fixed === true ? 'left' : col.fixed"
                     :sortable="col.sortable ? (api ? 'custom' : true) : false">
      <template #default="scope">
        <slot v-if="$slots[col.field]" :name="col.field" :row="scope.row" :value="scope.row[col.field]"/>
        <template v-else-if="col.type === 'actions'">
          <div class="op-actions" v-if="crudMode && !$slots.actions">
            <el-button size="small" text @click="openView(scope.row)">查看</el-button>
            <el-button type="primary" size="small" text @click="openEdit(scope.row)">编辑</el-button>
            <el-button type="danger" size="small" text @click="remove(scope.row)">删除</el-button>
          </div>
          <div class="op-actions" v-else-if="$slots.actions">
            <slot name="actions" :row="scope.row"/>
          </div>
        </template>
        <span v-else-if="col.formatter">{{ col.formatter(scope.row, scope.row[col.field]) }}</span>
        <span v-else>{{ scope.row[col.field] }}</span>
      </template>
    </el-table-column>
  </el-table>

  <div v-if="total > 0" class="pager">
    <el-pagination
        :current-page="page" :page-size="pageSize" :total="total"
        :page-sizes="pageSizeOptions"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
    />
  </div>

  <DataFormDialog
      v-if="crudMode"
      v-model="showDialog"
      :title="dialogMode === 'create' ? `新建${title}` : `编辑${title}`"
      :mode="dialogMode"
      :fields="fields ?? []"
      :form="form"
      :saving="saving"
      @save="save"
  />

  <DataFormDialog
      v-if="crudMode"
      v-model="showView"
      :title="`查看${title}`"
      mode="view"
      :fields="fields ?? []"
      :form="form"
  />
</template>

<style scoped>
/* 工具栏分两行：搜索行在上，插槽操作（如"创建"按钮）另起一行在下 */
.datatable-toolbar {
  flex-wrap: wrap;
  align-items: flex-start;
  row-gap: var(--space-sm);
}

.datatable-toolbar .toolbar-search {
  order: 1;
  flex: 1 1 100%;
}

.datatable-toolbar .toolbar-left {
  order: 2;
  flex: 1 1 100%;
  display: flex;
  justify-content: flex-start;
}

.datatable {
  width: 100%;
}

.pager {
  margin-top: var(--space-sm);
  display: flex;
  justify-content: flex-end;
}
</style>
