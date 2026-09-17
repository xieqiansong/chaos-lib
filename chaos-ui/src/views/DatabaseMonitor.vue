<template>
  <div class="db-monitor">
    <el-row :gutter="16" class="overview">
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="metric-label">数据库类型</div>
          <div class="metric-value">{{ overview.dbType || '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="metric-label">版本</div>
          <div class="metric-value text-sm">{{ overview.version || '-' }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="metric-label">总大小</div>
          <div class="metric-value">{{ formatBytes(overview.totalBytes) }}</div>
          <div v-if="!overview.sizeSupported" class="metric-sub">单表大小不可用（SQLite 缺 dbstat）</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="metric-label">表数量 / 总行数</div>
          <div class="metric-value">{{ overview.tableCount }} / {{ overview.totalRows.toLocaleString() }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="table-card">
      <template #header>
        <div class="table-header">
          <span>数据表（{{ tableData.length }}）</span>
          <div class="table-tools">
            <el-input v-model="keyword" placeholder="筛选表名" clearable style="width: 11.25rem; margin-left: 0.5rem" />
            <el-button style="margin-left: 0.5rem" :icon="Refresh" circle @click="loadOverview(); loadTables()" />
          </div>
        </div>
      </template>
      <el-table :data="filteredTables" stripe height="480" highlight-current-row @row-click="openDetail">
        <el-table-column prop="name" label="表名" sortable min-width="180" />
        <el-table-column label="行数" sortable :sort-method="sortNum('rows')" align="right" min-width="130">
          <template #default="{ row }">
            {{ row.rows.toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column label="表大小" sortable :sort-method="sortNum('tableBytes')" align="right" min-width="120">
          <template #default="{ row }">{{ row.sizeSupported ? formatBytes(row.tableBytes) : 'N/A' }}</template>
        </el-table-column>
        <el-table-column label="索引大小" sortable :sort-method="sortNum('indexBytes')" align="right" min-width="120">
          <template #default="{ row }">{{ row.sizeSupported ? formatBytes(row.indexBytes) : 'N/A' }}</template>
        </el-table-column>
        <el-table-column label="总大小" sortable :sort-method="sortNum('totalBytes')" align="right" min-width="120">
          <template #default="{ row }">{{ row.sizeSupported ? formatBytes(row.totalBytes) : 'N/A' }}</template>
        </el-table-column>
        <el-table-column prop="indexCount" label="索引数" align="center" width="90" />
      </el-table>
    </el-card>

    <el-drawer v-model="drawer" :title="`表详情：${current}`" size="55%" @closed="detail = null">
      <div v-if="detail">
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

        <h4>列（{{ detail.columns.length }}）</h4>
        <el-table :data="detail.columns" size="small" stripe max-height="260">
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

        <h4>索引（{{ detail.indexes.length }}）</h4>
        <el-table :data="detail.indexes" size="small" stripe max-height="200">
          <el-table-column prop="name" label="索引名" min-width="160" />
          <el-table-column label="唯一" width="70">
            <template #default="{ row }">{{ row.unique ? '是' : '否' }}</template>
          </el-table-column>
          <el-table-column prop="columns" label="列" min-width="160" />
        </el-table>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { getDbOverview, getTables, getTableDetail } from '@/utils/api'

interface TableStat {
  name: string
  rows: number
  rowsEstimated: boolean
  tableBytes: number
  indexBytes: number
  totalBytes: number
  indexCount: number
  sizeSupported: boolean
  seqScan?: number | null
  idxScan?: number | null
  lastVacuum?: string | null
  lastAnalyze?: string | null
}
interface ColumnInfo { name: string; type: string; nullable: boolean; isPk: boolean; default?: string | null }
interface IndexInfo { name: string; unique: boolean; columns: string }
interface TableDetail extends TableStat { columns: ColumnInfo[]; indexes: IndexInfo[] }
interface Overview { dbType: string; version: string; totalBytes: number; tableCount: number; totalRows: number; sizeSupported: boolean }

const overview = ref<Overview>({ dbType: '', version: '', totalBytes: 0, tableCount: 0, totalRows: 0, sizeSupported: false })
const tableData = ref<TableStat[]>([])
const keyword = ref('')
const drawer = ref(false)
const current = ref('')
const detail = ref<TableDetail | null>(null)

function formatBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(u.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  return (n / Math.pow(1024, i)).toFixed(i ? 1 : 0) + ' ' + u[i]
}
function sortNum(key: keyof TableStat) {
  return (a: TableStat, b: TableStat) => (a[key] as number) - (b[key] as number)
}
const filteredTables = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return tableData.value
  return tableData.value.filter(t => t.name.toLowerCase().includes(k))
})

async function loadOverview() {
  try {
    overview.value = await getDbOverview()
  } catch (e) {
    console.error(e)
  }
}
async function loadTables() {
  try {
    const data = await getTables()
    tableData.value = data.items || []
  } catch (e) {
    console.error(e)
  }
}
async function openDetail(row: TableStat) {
  current.value = row.name
  detail.value = null
  drawer.value = true
  try {
    detail.value = await getTableDetail(row.name)
  } catch (e) {
    console.error(e)
  }
}

onMounted(() => {
  loadOverview()
  loadTables()
})
</script>

<style scoped>
.overview {
  margin-bottom: 16px;
}
.metric-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.metric-value {
  font-size: 22px;
  font-weight: 600;
  margin-top: 6px;
}
.metric-value.text-sm {
  font-size: 14px;
  font-weight: 500;
}
.metric-sub {
  font-size: 12px;
  color: var(--el-color-warning);
  margin-top: 4px;
}
.table-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.table-tools {
  display: flex;
  align-items: center;
}
h4 {
  margin: 16px 0 8px;
}
</style>
