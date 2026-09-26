<script setup lang="ts">
// 配置驱动的通用表格 + 增删改查组件。
// 由 columns 派生表格列；searchable 列自动生成搜索栏；支持服务端(api)/本地(data)两种分页模式。
// 自定义单元格优先用具名插槽 #field，其次 formatter，否则纯文本。
// 操作列：传入完整 RestApi（含 create/update/remove）+ fields 时，自动渲染「查看/编辑/删除」并内置弹窗；
//         业务扩展动作经 #actions 插槽追加在同格（如定时任务的「运行 / 历史」），
//         非 CRUD 场景可仅用 { type: 'actions' } + #actions 插槽自行定义全部按钮。
import {computed, onMounted, reactive, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'

// 选择变化事件：透传 el-table 的 selection-change，便于业务层做批量操作。
const emit = defineEmits<{
  (e: 'selection-change', rows: any[]): void
  (e: 'reset'): void
  (e: 'row-click', row: any, column: any, event: any): void
  (e: 'loaded', rows: any[]): void
}>()
import {format, parseISO} from 'date-fns'
import {useDataTable} from '@/composables/useDataTable'
import {defaultFieldValue} from '@/composables/useFormDefaults'
import {type RestApi} from '@/composables/useRestApi'
import type {DataTableApiParams, DataTableApiResult, DataTableColumn, CrudAction, FormField} from '@/components/dataTable/types'
import {CRUD_ACTION} from '@/components/dataTable/types'
import DataFormDialog from '@/components/DataFormDialog.vue'

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
      /**
       * 启用（显示）的内置 CRUD 动作，默认全开：[CREATE, VIEW, EDIT, DELETE]。
       * 业务只需子集时传入，如 [VIEW, EDIT, DELETE]（平铺表单隐藏「新建」）。
       * 取值见 dataTable/types 的 CRUD_ACTION。
       */
      enabledActions?: CrudAction[]
      stripe?: boolean
      border?: boolean
      /** type=switch 列切换时的回调：返回 Promise，成功后自动刷新并提示；未提供则开关禁用 */
      switchHandler?: (row: any, next: boolean) => Promise<void> | void
      /** 初始排序：首次取数即带 sort 参数（后端默认按 id 排，需要按业务字段排时由此指定） */
      defaultSort?: { field: string; order: 'ascending' | 'descending' }
      /** 行 class 回调（透传 el-table 的 row-class-name），用于按行状态高亮，如逾期/已到点 */
      rowClassName?: (row: any, index: number) => string
      /** 勾选列可勾选判定（透传 el-table-column selection 的 :selectable） */
      selectable?: (row: any, index: number) => boolean
      /** 勾选列是否跨分页保留选中（透传 :reserve-selection） */
      reserveSelection?: boolean
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
      enabledActions: () => [CRUD_ACTION.CREATE, CRUD_ACTION.VIEW, CRUD_ACTION.EDIT, CRUD_ACTION.DELETE],
    },
)

// api 为完整 RestApi 对象时启用内置 CRUD；仅传取数函数则仅做表格。
const isCrud = computed(() => !!props.api && typeof props.api !== 'function')
const crudApi = computed(() => (isCrud.value ? (props.api as RestApi<any>) : undefined))
const fetchFn = computed(() =>
    typeof props.api === 'function' ? props.api : props.api ? props.api.fetch : undefined,
)
const crudMode = computed(() => isCrud.value && !!props.fields)

// 内置 CRUD 动作是否启用（被 template 逐项显隐使用），取值见 CRUD_ACTION 枚举
function hasAction(action: CrudAction): boolean {
  return props.enabledActions.includes(action)
}

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
  defaultSort: props.defaultSort,
})

onMounted(async () => {
  await getData()
  emit('loaded', tableData.value)
})

// 重置：先通知父级清空自定义筛选（如提前查询/计划树），再清空搜索栏内置项并刷新
function onReset() {
  emit('reset')
  handleReset()
}

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
    f[field.field] = defaultFieldValue(field)
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

// 行内开关：type=switch 的列交由外部 switchHandler 处理；成功后刷新并提示，失败提示。
// 具体调用哪个接口由消费方决定（标准 CRUD 基线不含状态切换），保持组件通用。
async function onSwitchChange(row: any, next: boolean) {
  if (!props.switchHandler) return
  try {
    await props.switchHandler(row, next)
    ElMessage.success('状态已更新')
    refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '更新状态失败')
  }
}

// 行内时间：type=datetime 的列按标准格式序列化显示（fmt 遵循 date-fns 的 token）
function formatDateTime(value: any, fmt = 'yyyy-MM-dd HH:mm:ss'): string {
  if (value === null || value === undefined || value === '') return ''
  const d = value instanceof Date ? value : parseISO(value)
  if (isNaN(d.getTime())) return String(value)
  return format(d, fmt)
}

defineExpose({refresh, getData})
</script>

<template>
  <section class="section-toolbar datatable-toolbar">
    <div v-if="$slots.toolbar || (crudMode && hasAction(CRUD_ACTION.CREATE))" class="toolbar-left">
      <slot name="toolbar"/>
      <el-button v-if="crudMode && hasAction(CRUD_ACTION.CREATE)" size="small" type="primary" @click="openCreate">+ 新建{{ title }}</el-button>
    </div>
    <div v-if="searchableColumns.length || $slots['search-extra']" class="toolbar-search">
      <el-form :inline="true" @submit.prevent>
        <slot name="search-extra" />
        <el-form-item
          v-for="col in searchableColumns"
          :key="col.field"
          :label="col.search?.label ?? col.title"
        >
          <el-select
            v-if="col.search?.type === 'select'"
            v-model="searchState[col.field]"
            :placeholder="col.search?.placeholder ?? '请选择'"
            clearable
            size="small"
            style="width: 160px"
          >
            <el-option
              v-for="o in (col.search?.options ?? [])"
              :key="o.value"
              :label="o.label"
              :value="o.value"
            />
          </el-select>
          <el-input
            v-else
            v-model="searchState[col.field]"
            :placeholder="col.search?.placeholder ?? '请输入'"
            clearable
            style="width: 200px"
            size="small"
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="small" @click="handleSearch">查询</el-button>
          <el-button size="small" @click="onReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>
  </section>

  <el-table
    v-loading="loading"
    :data="tableData"
    :row-key="rowKey"
    :tree-props="treeProps"
    :stripe="stripe"
    :border="border"
    class="datatable"
    :default-sort="defaultSort ? {prop: defaultSort.field, order: defaultSort.order} : undefined"
    :row-class-name="rowClassName"
    size="small"
    @sort-change="handleSortChange"
    @selection-change="(rows: any) => emit('selection-change', rows)"
    @row-click="(row: any, column: any, event: any) => emit('row-click', row, column, event)"
  >
    <el-table-column
      v-if="selection"
      type="selection"
      width="48"
      :selectable="selectable"
      :reserve-selection="reserveSelection"
    />

    <el-table-column
      v-for="col in finalColumns"
      :key="col.field"
      :prop="col.field"
      :label="col.title"
      :width="col.width"
      :min-width="col.minWidth"
      :align="col.align"
      :fixed="col.fixed === true ? 'left' : col.fixed"
      :sortable="col.sortable ? (api ? 'custom' : true) : false"
    >
      <template #default="scope">
        <slot v-if="$slots[col.field]" :name="col.field" :row="scope.row" :value="scope.row[col.field]"/>
        <template v-else-if="col.type === 'actions'">
          <!-- 内置增删改按钮 + 业务自定义按钮（#actions 插槽）共存：
               插槽只负责「扩展动作」，标准动作仍由组件统一提供，避免业务页重复实现。 -->
          <div class="op-actions">
            <template v-if="crudMode">
              <el-button v-if="hasAction(CRUD_ACTION.VIEW)" size="small" text @click="openView(scope.row)">查看</el-button>
              <el-button v-if="hasAction(CRUD_ACTION.EDIT)" type="primary" size="small" text @click="openEdit(scope.row)">编辑</el-button>
              <el-button v-if="hasAction(CRUD_ACTION.DELETE)" type="danger" size="small" text @click="remove(scope.row)">删除</el-button>
            </template>
            <slot name="actions" :row="scope.row"/>
          </div>
        </template>
        <el-switch
            v-else-if="col.type === 'switch'"
            :model-value="scope.row[col.field]"
            :disabled="!props.switchHandler"
            @update:model-value="(val: boolean) => onSwitchChange(scope.row, val)"
        />
        <span v-else-if="col.formatter">{{ col.formatter(scope.row, scope.row[col.field]) }}</span>
        <span v-else-if="col.type === 'datetime'">{{ formatDateTime(scope.row[col.field], col.datetimeFormat) }}</span>
        <span v-else>{{ scope.row[col.field] }}</span>
      </template>
    </el-table-column>
  </el-table>

  <div v-if="total > 0" class="pager">
    <el-pagination
      :current-page="page"
      :page-size="pageSize"
      :total="total"
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
    v-if="crudMode && hasAction(CRUD_ACTION.VIEW)"
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

/* 搜索项自带 18px 下边距，会与工具栏 row-gap 叠加成过宽的间隙；
   改为 flex 布局 + row-gap，行间距统一由 row-gap 控制。 */
.datatable-toolbar .toolbar-search :deep(.el-form) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  row-gap: var(--space-sm);
}

.datatable-toolbar .toolbar-search :deep(.el-form-item) {
  margin-bottom: 0;
}

.datatable-toolbar .toolbar-left {
  order: 2;
  flex: 1 1 100%;
  display: flex;
  align-items: center;
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
