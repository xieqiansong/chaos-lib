// DataTable 的组合式逻辑：统一管理分页、搜索、排序与服务端/本地两种数据模式。
// 视图组件（DataTable.vue）只负责渲染，状态与取数逻辑全部收敛在此。
import {computed, reactive, ref} from 'vue'
import type {
  DataTableApiParams,
  DataTableApiResult,
  DataTableColumn,
} from '@/components/dataTable/types'

interface UseDataTableProps {
  columns: DataTableColumn[]
  data?: any[]
  api?: (params: DataTableApiParams) => Promise<DataTableApiResult>
  pageSize?: number
  pageSizeOptions?: number[]
  /** 初始排序：首次取数即带上 sort 参数，无需用户点表头 */
  defaultSort?: { field: string; order: 'ascending' | 'descending' }
}

export function useDataTable(props: UseDataTableProps) {
  const page = ref(1)
  const pageSize = ref(props.pageSize ?? 20)
  const total = ref(0)
  const rows = ref<any[]>([])
  const loading = ref(false)
  const searchState = reactive<Record<string, any>>({})
  const sortState = ref<{ field: string; order: 'ascending' | 'descending' | null } | null>(
      props.defaultSort ? {field: props.defaultSort.field, order: props.defaultSort.order} : null,
  )

  const apiMode = computed(() => !!props.api)
  const searchableColumns = computed(() => props.columns.filter((c) => c.searchable))
  const renderColumns = computed(() => props.columns)

  // 初始化搜索字段默认值
  searchableColumns.value.forEach((c) => {
    if (!(c.field in searchState)) searchState[c.field] = ''
  })

  const tableData = computed(() => {
    if (apiMode.value) return rows.value
    // 本地模式：先按搜索过滤，再分页（排序交给 el-table 内置处理）
    let list = props.data ?? []
    const active = searchableColumns.value.filter((c) => {
      const v = searchState[c.field]
      return v !== '' && v !== null && v !== undefined
    })
    if (active.length) {
      list = list.filter((row) =>
        active.every((c) =>
          String(row[c.field] ?? '')
            .toLowerCase()
            .includes(String(searchState[c.field]).toLowerCase()),
        ),
      )
    }
    total.value = list.length
    const start = (page.value - 1) * pageSize.value
    return list.slice(start, start + pageSize.value)
  })

  async function getData() {
    if (!apiMode.value) return
    loading.value = true
    try {
      const res = await props.api!({
        page: page.value,
        pageSize: pageSize.value,
        search: {...searchState},
        sort: sortState.value ?? undefined,
      })
      rows.value = res.rows ?? []
      total.value = res.total ?? 0
      // 当前页被取空且非首页（删除后数据变少），回退一页再拉取
      if (rows.value.length === 0 && page.value > 1) {
        page.value -= 1
        return getData()
      }
    } finally {
      loading.value = false
    }
  }

  function handleSearch() {
    page.value = 1
    getData()
  }

  function handleReset() {
    searchableColumns.value.forEach((c) => (searchState[c.field] = ''))
    page.value = 1
    getData()
  }

  function handlePageChange(p: number) {
    page.value = p
    getData()
  }

  function handleSizeChange(s: number) {
    pageSize.value = s
    page.value = 1
    getData()
  }

  function handleSortChange({
    prop,
    order,
  }: {
    prop: string
    order: 'ascending' | 'descending' | null
  }) {
    sortState.value = order ? {field: prop, order} : null
    if (apiMode.value) getData()
  }

  function refresh() {
    getData()
  }

  return {
    page,
    pageSize,
    total,
    loading,
    searchState,
    searchableColumns,
    renderColumns,
    tableData,
    getData,
    handleSearch,
    handleReset,
    handlePageChange,
    handleSizeChange,
    handleSortChange,
    refresh,
  }
}
