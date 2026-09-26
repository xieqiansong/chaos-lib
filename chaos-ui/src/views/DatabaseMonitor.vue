<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">数据库监控</span>
      <div class="metric-strip">
        <div class="metric-chip">
          <span class="metric-label">数据库类型</span>
          <span class="metric-value">{{ overview.dbType || '—' }}</span>
        </div>
        <div class="metric-chip">
          <span class="metric-label">版本</span>
          <span class="metric-value text-sm truncate">{{ overview.version || '—' }}</span>
        </div>
        <div class="metric-chip">
          <span class="metric-label">总大小</span>
          <span class="metric-value">{{ formatBytes(overview.totalBytes) }}</span>
          <el-tooltip v-if="!overview.sizeSupported" content="单表大小不可用（SQLite 缺 dbstat）" placement="top">
            <el-icon class="warning-icon"><Warning /></el-icon>
          </el-tooltip>
        </div>
        <div class="metric-chip">
          <span class="metric-label">表 / 行</span>
          <span class="metric-value">{{ overview.tableCount }} / {{ overview.totalRows.toLocaleString() }}</span>
        </div>
      </div>
      <div class="section-actions">
        <el-button size="small" :icon="Refresh" :loading="loading" @click="refreshAll">刷新</el-button>
      </div>
    </div>

    <el-alert
      v-if="error"
      type="error"
      :message="error"
      show-icon
      class="mb-sm"
      @close="error = ''"
    />

    <!-- 列表：复用 DataTable 的配置驱动引擎（fetch-only 模式，无 CRUD）。
         分页 / 按表名搜索 / 排序全部由 DataTable + 后端分页接口承担；
         「查看」操作经 #actions 插槽触发页面内的详情抽屉。 -->
    <DataTable
      ref="tableRef"
      :columns="columns"
      :api="fetchTables"
      :page-size="20"
      row-key="name"
    >
      <template #actions="{ row }">
        <el-button size="small" text type="primary" @click="openDetail(row)">查看</el-button>
      </template>
    </DataTable>

    <el-drawer v-model="drawer" :title="`表详情：${current}`" size="55%" @closed="detail = null">
      <div v-loading="detailLoading">
        <template v-if="detail">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="行数">
              {{ detail.rows.toLocaleString() }}
            </el-descriptions-item>
            <el-descriptions-item label="索引数">{{ detail.indexCount }}</el-descriptions-item>
            <el-descriptions-item label="表大小">{{ detail.sizeSupported ? formatBytes(detail.tableBytes) : 'N/A' }}</el-descriptions-item>
            <el-descriptions-item label="索引大小">{{ detail.sizeSupported ? formatBytes(detail.indexBytes) : 'N/A' }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.seqScan !== null && detail.seqScan !== undefined" label="顺序扫描">{{ detail.seqScan.toLocaleString() }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.idxScan !== null && detail.idxScan !== undefined" label="索引扫描">{{ detail.idxScan.toLocaleString() }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.lastVacuum" label="最近 Vacuum">{{ detail.lastVacuum }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.lastAnalyze" label="最近 Analyze">{{ detail.lastAnalyze }}</el-descriptions-item>
          </el-descriptions>

          <h4>列（{{ detailColumns.length }}）</h4>
          <el-table :data="detailColumns" size="small" stripe max-height="260">
            <el-table-column prop="name" label="列名" min-width="120" />
            <el-table-column prop="type" label="类型" min-width="120" />
            <el-table-column label="可空" width="70">
              <template #default="{ row }">{{ row.nullable ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column label="主键" width="70">
              <template #default="{ row }">{{ row.isPk ? '★' : '' }}</template>
            </el-table-column>
            <el-table-column prop="default" label="默认值" min-width="120" />
          </el-table>

          <h4>索引（{{ detailIndexes.length }}）</h4>
          <el-table :data="detailIndexes" size="small" stripe max-height="200">
            <el-table-column prop="name" label="索引名" min-width="160" />
            <el-table-column label="唯一" width="70">
              <template #default="{ row }">{{ row.unique ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column prop="columns" label="列" min-width="160" />
          </el-table>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Warning } from '@element-plus/icons-vue'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, DataTableApiParams, DataTableApiResult} from '@/components/dataTable/types'
import {
  getDbOverview,
  getTables,
  getTableDetail,
  type DbOverview,
  type TableStat,
  type TableDetail,
  type ColumnInfo,
  type IndexInfo,
} from '@/api/dbMonitor'

// ── 概览指标条（页面级，独立于分页列表）──
const overview = ref<DbOverview>({ dbType: '', version: '', totalBytes: 0, tableCount: 0, totalRows: 0, sizeSupported: false })
const loading = ref(false)
const error = ref('')

// ── 详情抽屉 ──
const drawer = ref(false)
const current = ref('')
const detail = ref<TableDetail | null>(null)
const detailLoading = ref(false)
const detailColumns = computed<ColumnInfo[]>(() => detail.value?.columns ?? [])
const detailIndexes = computed<IndexInfo[]>(() => detail.value?.indexes ?? [])

// ── 列配置：驱动 DataTable 的表头 / 搜索栏 / 排序 ──
const columns: DataTableColumn[] = [
  {field: 'name', title: '表名', minWidth: 180, searchable: true},
  {field: 'rows', title: '行数', width: 130, sortable: true, align: 'right', formatter: (r: any) => (r.rows ?? 0).toLocaleString()},
  {field: 'tableBytes', title: '表大小', width: 120, sortable: true, align: 'right', formatter: (r: any) => r.sizeSupported ? formatBytes(r.tableBytes) : 'N/A'},
  {field: 'indexBytes', title: '索引大小', width: 120, sortable: true, align: 'right', formatter: (r: any) => r.sizeSupported ? formatBytes(r.indexBytes) : 'N/A'},
  {field: 'totalBytes', title: '总大小', width: 120, sortable: true, align: 'right', formatter: (r: any) => r.sizeSupported ? formatBytes(r.totalBytes) : 'N/A'},
  {field: 'indexCount', title: '索引数', width: 90, align: 'center'},
  {field: '__actions', title: '操作', width: 100, type: 'actions', fixed: 'right'},
]

// 字段名 → snake_case（与标准 CRUD 基线的 toSnake 约定对齐，供 sort 参数使用）
function toSnake(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
}

// DataTable(fetch-only) 的取数函数：翻译分页 / 搜索 / 排序参数 → 后端分页接口。
async function fetchTables(params: DataTableApiParams): Promise<DataTableApiResult> {
  const query: Record<string, any> = {page: params.page, size: params.pageSize}
  if (params.sort?.field) {
    query.sort = toSnake(params.sort.field)
    query.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
  }
  for (const [k, v] of Object.entries(params.search || {})) {
    if (v !== '' && v !== null && v !== undefined) query[k] = v
  }
  const res = await getTables(query)
  return {rows: res.items ?? [], total: res.total ?? 0}
}

function formatBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(u.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  return (n / Math.pow(1024, i)).toFixed(i ? 1 : 0) + ' ' + u[i]
}

async function loadOverview() {
  try {
    overview.value = await getDbOverview()
  } catch (e) {
    error.value = '加载数据库概览失败：' + (e instanceof Error ? e.message : String(e))
    console.error(e)
  }
}

const tableRef = ref<InstanceType<typeof DataTable> | null>(null)

// 刷新：概览 + 列表一起刷新（列表由 DataTable 暴露的 refresh 负责）。
async function refreshAll() {
  loading.value = true
  error.value = ''
  try {
    await loadOverview()
    tableRef.value?.refresh()
  } finally {
    loading.value = false
  }
}

async function openDetail(row: TableStat) {
  current.value = row.name
  detail.value = null
  detailLoading.value = true
  drawer.value = true
  try {
    detail.value = await getTableDetail(row.name)
  } catch (e) {
    ElMessage.error('加载表详情失败：' + (e instanceof Error ? e.message : String(e)))
    console.error(e)
  } finally {
    detailLoading.value = false
  }
}

onMounted(() => {
  // 列表由 DataTable 自行 onMounted 取数；这里仅加载概览。
  loadOverview()
})
</script>

<style scoped>
.metric-strip {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
}
.metric-chip {
  display: flex;
  align-items: center;
  gap: var(--space-05);
  min-width: 0;
  white-space: nowrap;
}
.metric-label {
  color: var(--el-text-color-secondary);
  font-size: var(--el-font-size-small);
  flex-shrink: 0;
}
.metric-value {
  color: var(--el-text-color-primary);
  font-size: var(--el-font-size-base);
  font-weight: 600;
  min-width: 0;
}
.metric-value.text-sm {
  font-size: var(--el-font-size-small);
  font-weight: 500;
}
.warning-icon {
  color: var(--el-color-warning);
  font-size: var(--el-font-size-small);
  cursor: help;
  flex-shrink: 0;
}
h4 {
  margin: var(--space-xl) 0 var(--space-sm);
  color: var(--el-text-color-primary);
  font-weight: 600;
}
</style>
