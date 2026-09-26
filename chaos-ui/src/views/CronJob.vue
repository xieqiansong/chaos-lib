<script setup lang="ts">
// 定时任务界面：参考标准数据，由通用 DataTable（内置查看/编辑/删除 + 弹窗）驱动完整 CRUD。
// 仅声明 columns 与 fields 两份配置即可，无需任何增删改查样板。
//
// 本资源相对标准基线有四个「自定义」点（均属扩展能力，不污染基线）：
//   1. 启停有副作用（挂载 / 摘除 cron 调度），走自定义 /:id/status 接口，由 switch-handler 注入。
//   2. 「运行 / 历史」是标准 CRUD 之外的动作，经 #actions 插槽追加到操作列。
//   3. 下次执行时间（NextRun）由后端按 cron 表达式实时计算，不落库 —— 前端只负责渲染。
//   4. 工具栏的 cron 表达式校验沿用自定义 /preview 接口，避免填错表达式却要到点才发现。
import {ref, watch} from 'vue'
import {ElMessage} from 'element-plus'
import {format as formatDate, parseISO} from 'date-fns'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {cronJobApi, type CronJob, type CronJobRun} from '@/api/cronJob'

// 兼容 App.vue 向动态视图透传的 search-text（本页用 DataTable 自带搜索，故未使用）
defineProps<{ searchText?: string }>()

const actionTypeMap: Record<string, { text: string; type: string }> = {
  http: {text: 'HTTP', type: 'primary'},
  shell: {text: '命令', type: 'warning'},
}

const statusMap: Record<string, { text: string; type: string }> = {
  ok: {text: '成功', type: 'success'},
  failed: {text: '失败', type: 'danger'},
  '': {text: '—', type: 'info'},
}

const columns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 150, searchable: true},
  {field: 'CronExpr', title: 'Cron', width: 150, searchable: true, search: {placeholder: '如 */5 * * * *'}},
  {
    field: 'ActionType', title: '动作', width: 80, searchable: true,
    search: {type: 'select', options: [{label: 'HTTP', value: 'http'}, {label: '命令', value: 'shell'}]},
  },
  {field: 'ActionConfig', title: '动作配置', minWidth: 200},
  {field: 'Enabled', title: '启用', width: 80, type: 'switch'},
  {field: 'TimeoutSec', title: '超时(秒)', width: 90, align: 'right'},
  {field: 'LastStatus', title: '上次结果', width: 100},
  {field: 'LastRunAt', title: '上次执行', width: 170, type: 'datetime'},
  {field: 'NextRun', title: '下次执行', width: 170, type: 'datetime'},
  // 显式声明操作列以放宽宽度：内置「查看/编辑/删除」+ #actions 插槽的「运行/历史」
  {field: '__actions', title: '操作', width: 230, type: 'actions', fixed: 'right'},
]

// 表单字段配置（与 columns 对应，驱动 DataTable 内置的 DataFormDialog）
const fields: FormField[] = [
  {field: 'Name', title: '任务名称', type: 'text', span: 12, required: true, placeholder: '请输入任务名称'},
  {
    field: 'ActionType', title: '动作类型', type: 'select', span: 12, defaultValue: 'http',
    options: [
      {label: 'HTTP 请求 / Webhook', value: 'http'},
      {label: '执行命令 / 脚本（预留）', value: 'shell'},
    ],
  },
  {
    field: 'CronExpr', title: 'Cron', type: 'text', required: true,
    placeholder: '5 字段(分 时 日 月 周)如 */5 * * * *；6 字段含秒如 */30 * * * * *',
  },
  {
    field: 'ActionConfig', title: '动作配置(JSON)', type: 'json', rows: 4,
    placeholder: 'http: {"method":"POST","url":"/api/systemJobs/sweep"}；shell: {"command":"echo hi"}',
  },
  {field: 'TimeoutSec', title: '超时(秒)', type: 'number', span: 8, min: 1, defaultValue: 30},
  {field: 'Enabled', title: '启用', type: 'switch', span: 8, defaultValue: true},
]

const tableRef = ref<InstanceType<typeof DataTable> | null>(null)

// 启停：交给 cronJobApi 的自定义 setStatus（DataTable 负责刷新与提示）
function onEnabledChange(row: CronJob, next: boolean) {
  return cronJobApi.setStatus(row.ID, next)
}

// 立即执行一次（标准 CRUD 之外的动作）
async function runJob(row: CronJob) {
  try {
    const run = await cronJobApi.run(row.ID)
    if (run?.Success) ElMessage.success('执行成功')
    else ElMessage.warning('执行失败，可查看运行历史')
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '触发失败')
  }
}

// 运行历史（标准 CRUD 之外的只读弹窗）
const showHistory = ref(false)
const historyJobName = ref('')
const runs = ref<CronJobRun[]>([])
const runsLoading = ref(false)

async function openHistory(row: CronJob) {
  historyJobName.value = row.Name
  showHistory.value = true
  runsLoading.value = true
  try {
    const res = await cronJobApi.runs(row.ID, {page: 1, size: 50})
    runs.value = res?.items ?? []
  } catch (e: any) {
    ElMessage.error(e?.message || '加载历史失败')
    runs.value = []
  } finally {
    runsLoading.value = false
  }
}

// cron 表达式校验（工具栏）：输入后防抖调用 /preview
const previewExpr = ref('')
const previewValid = ref<boolean | null>(null)
const previewError = ref('')
const previewRuns = ref<string[]>([])
let previewTimer: ReturnType<typeof setTimeout> | null = null

async function runPreview() {
  const expr = previewExpr.value.trim()
  if (!expr) {
    previewValid.value = null
    previewError.value = ''
    previewRuns.value = []
    return
  }
  try {
    const res = await cronJobApi.preview(expr, 5)
    previewValid.value = !!res?.valid
    previewError.value = res?.valid ? '' : (res?.error || '表达式无效')
    previewRuns.value = (res?.nextRuns ?? []).map((s: string) => fmtTime(s))
  } catch (e: any) {
    previewValid.value = false
    previewError.value = e?.message || '校验失败'
    previewRuns.value = []
  }
}

watch(previewExpr, () => {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(runPreview, 300)
})

function fmtTime(s: string | null): string {
  if (!s) return '—'
  const d = parseISO(s)
  if (isNaN(d.getTime())) return s
  return formatDate(d, 'yyyy-MM-dd HH:mm:ss')
}

function duration(start: string, end: string | null): string {
  const s = parseISO(start).getTime()
  const e = end ? parseISO(end).getTime() : Date.now()
  const ms = Math.max(0, e - s)
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(1) + 's'
}

// 动作配置（JSON 文本）在表格里的摘要：http 显示「方法 URL」，命令显示命令本身
function actionSummary(row: CronJob): string {
  try {
    const cfg = JSON.parse(row.ActionConfig || '{}')
    if (row.ActionType === 'http') return `${cfg.method || 'GET'} ${cfg.url || ''}`.trim()
    return cfg.command || row.ActionConfig
  } catch {
    return row.ActionConfig
  }
}
</script>

<template>
  <div class="cronjob-view">
    <DataTable
        ref="tableRef"
        :columns="columns"
        :api="cronJobApi"
        :fields="fields"
        title="定时任务"
        row-key="ID"
        :switch-handler="onEnabledChange"
    >

      <template #ActionType="{ row }">
        <el-tag size="small" :type="(actionTypeMap[row.ActionType]?.type as any) || 'info'">
          {{ actionTypeMap[row.ActionType]?.text || row.ActionType }}
        </el-tag>
      </template>

      <template #ActionConfig="{ row }">
        <span class="text-secondary text-xs break-all">{{ actionSummary(row) }}</span>
      </template>

      <template #LastStatus="{ row }">
        <el-tag size="small" :type="(statusMap[row.LastStatus ?? '']?.type as any) || 'info'">
          {{ statusMap[row.LastStatus ?? '']?.text || row.LastStatus }}
        </el-tag>
      </template>

      <template #NextRun="{ row }">
        <span v-if="!row.Enabled" class="text-secondary">已停用</span>
        <span v-else>{{ fmtTime(row.NextRun) }}</span>
      </template>

      <template #actions="{ row }">
        <el-button size="small" type="primary" text @click="runJob(row)">运行</el-button>
        <el-button size="small" text @click="openHistory(row)">历史</el-button>
      </template>
    </DataTable>

    <el-dialog v-model="showHistory" :title="`运行历史 — ${historyJobName}`" width="46rem">
      <el-table :data="runs" v-loading="runsLoading" border stripe size="small" max-height="50vh">
        <el-table-column label="开始时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.StartedAt) }}</template>
        </el-table-column>
        <el-table-column label="耗时" width="90">
          <template #default="{ row }">{{ duration(row.StartedAt, row.FinishedAt) }}</template>
        </el-table-column>
        <el-table-column label="结果" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.Success ? 'success' : 'danger'">{{ row.Success ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="输出 / 错误">
          <template #default="{ row }">
            <pre class="run-output">{{ row.Success ? row.Output : (row.Error || row.Output) }}</pre>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<style scoped>
.cron-check {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

.cron-check .el-input {
  width: 16rem;
}

.run-output {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  max-height: 160px;
  overflow: auto;
  background: var(--el-fill-color-light, #f5f7fa);
  padding: 6px 8px;
  border-radius: 4px;
}
</style>
