<script setup lang="ts">
// 配置驱动的通用表格组件。
// 由 columns 派生表格列；searchable 列自动生成搜索栏；支持服务端(api)/本地(data)两种分页模式。
// 自定义单元格优先用具名插槽 #field，其次 formatter，否则纯文本。
// 操作列用 columns 中的 { type: 'actions' } + 具名插槽 #actions（作用域含 row）。
import {onMounted} from 'vue'
import {useDataTable} from '@/composables/useDataTable'
import type {DataTableColumn} from '@/components/dataTable/types'

const props = withDefaults(
    defineProps<{
      columns: DataTableColumn[]
      /** 本地模式：直接传入数组 */
      data?: any[]
      /** 服务端模式：传入取数函数（请求/响应约定见 types.ts） */
      api?: (params: any) => Promise<{ rows: any[]; total: number }>
      rowKey?: string
      pageSize?: number
      pageSizeOptions?: number[]
      selection?: boolean
      treeProps?: { children: string; hasChildren?: string }
      title?: string
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
      border: false,
    },
)

const {
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
} = useDataTable(props)

onMounted(() => getData())

// 暴露给父组件：刷新 / 取数
defineExpose({refresh, getData})
</script>

<template>
  <section class="section-toolbar datatable-toolbar">
    <div class="toolbar-left">
      <slot name="toolbar"/>
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

  <el-table
      v-loading="loading"
      :data="tableData"
      :row-key="rowKey"
      :tree-props="treeProps"
      :stripe="stripe"
      :border="border"
      class="datatable"
      @sort-change="handleSortChange"
  >
    <el-table-column v-if="selection" type="selection" width="48"/>

    <el-table-column
        v-for="col in renderColumns"
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
        <slot
            v-if="$slots[col.field]"
            :name="col.field"
            :row="scope.row"
            :value="scope.row[col.field]"
        />
        <template v-else-if="col.type === 'actions'">
          <div class="op-actions">
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
        :current-page="page"
        :page-size="pageSize"
        :total="total"
        :page-sizes="pageSizeOptions"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
    />
  </div>
</template>

<style scoped>
.datatable-toolbar {
  flex-wrap: wrap;
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
