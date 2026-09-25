<script setup lang="ts">
import {onMounted, ref, watch} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import {format as formatDate, parseISO} from 'date-fns'

interface CronJob {
  ID: number
  Name: string
  CronExpr: string
  ActionType: string
  ActionConfig: string
  Enabled: boolean
  TimeoutSec: number
  LastRunAt: string | null
  LastStatus: string
  NextRun: string | null
}

interface CronJobRun {
  ID: number
  JobID: number
  StartedAt: string
  FinishedAt: string | null
  Success: boolean
  Output: string
  Error: string
}

const jobs = ref<CronJob[]>([])
const loading = ref(false)

const showDialog = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

const formData = ref({
  Name: '',
  CronExpr: '',
  ActionType: 'http',
  Method: 'POST',
  URL: '',
  Headers: '',
  Body: '',
  Command: '',
  WorkDir: '',
  TimeoutSec: 30,
  Enabled: true,
})

// cron 表达式实时预览
const previewValid = ref(true)
const previewError = ref('')
const previewRuns = ref<string[]>([])
let previewTimer: any = null

const actionTypeMap: Record<string, {text: string; type: string}> = {
  http: {text: 'HTTP', type: 'primary'},
  shell: {text: '命令', type: 'warning'},
}

const statusMap: Record<string, {text: string; type: string}> = {
  ok: {text: '成功', type: 'success'},
  failed: {text: '失败', type: 'danger'},
  '': {text: '—', type: 'info'},
}

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

async function fetchJobs() {
  loading.value = true
  try {
    const res = await sendMessage('cronJobs', 'GET')
    jobs.value = Array.isArray(res) ? res : []
  } catch (e: any) {
    ElMessage.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  formData.value = {
    Name: '',
    CronExpr: '',
    ActionType: 'http',
    Method: 'POST',
    URL: '',
    Headers: '',
    Body: '',
    Command: '',
    WorkDir: '',
    TimeoutSec: 30,
    Enabled: true,
  }
  previewValid.value = true
  previewError.value = ''
  previewRuns.value = []
}

function openCreate() {
  editingId.value = null
  resetForm()
  showDialog.value = true
}

async function openEdit(row: CronJob) {
  editingId.value = row.ID
  resetForm()
  try {
    const job: any = await sendMessage(`cronJobs/${row.ID}`, 'GET')
    formData.value.Name = job.Name || ''
    formData.value.CronExpr = job.CronExpr || ''
    formData.value.ActionType = job.ActionType || 'http'
    formData.value.TimeoutSec = job.TimeoutSec ?? 30
    formData.value.Enabled = job.Enabled !== false
    try {
      const cfg = JSON.parse(job.ActionConfig || '{}')
      if (formData.value.ActionType === 'http') {
        formData.value.Method = cfg.method || 'POST'
        formData.value.URL = cfg.url || ''
        formData.value.Headers = cfg.headers ? JSON.stringify(cfg.headers, null, 2) : ''
        formData.value.Body = cfg.body || ''
      } else if (formData.value.ActionType === 'shell') {
        formData.value.Command = cfg.command || ''
        formData.value.WorkDir = cfg.workDir || ''
      }
    } catch {
      /* 配置解析失败忽略 */
    }
    showDialog.value = true
    runPreview()
  } catch (e: any) {
    ElMessage.error(e?.message || '读取失败')
  }
}

function parseHeaders(text: string): Record<string, string> {
  const t = (text || '').trim()
  if (!t) return {}
  try {
    const obj = JSON.parse(t)
    if (obj && typeof obj === 'object') return obj as Record<string, string>
  } catch {
    /* 退回按行解析 */
  }
  const obj: Record<string, string> = {}
  for (const line of t.split('\n')) {
    const idx = line.indexOf(':')
    if (idx > 0) obj[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
  }
  return obj
}

function buildActionConfig(): string {
  if (formData.value.ActionType === 'shell') {
    return JSON.stringify({
      command: formData.value.Command,
      args: [],
      workDir: formData.value.WorkDir,
    })
  }
  return JSON.stringify({
    method: formData.value.Method || 'POST',
    url: formData.value.URL,
    headers: parseHeaders(formData.value.Headers),
    body: formData.value.Body,
  })
}

async function runPreview() {
  const expr = formData.value.CronExpr.trim()
  if (!expr) {
    previewValid.value = true
    previewError.value = ''
    previewRuns.value = []
    return
  }
  try {
    const res: any = await sendMessage('cronJobs/preview', 'POST', {cronExpr: expr, count: 5})
    previewValid.value = !!res.valid
    previewError.value = res.valid ? '' : (res.error || '表达式无效')
    previewRuns.value = (res.nextRuns || []).map((s: string) => fmtTime(s))
  } catch (e: any) {
    previewValid.value = false
    previewError.value = e?.message || '校验失败'
    previewRuns.value = []
  }
}

watch(() => formData.value.CronExpr, () => {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(runPreview, 300)
})

async function submit() {
  if (!formData.value.Name.trim()) {
    ElMessage.error('请输入任务名称')
    return
  }
  if (!formData.value.CronExpr.trim()) {
    ElMessage.error('请输入 cron 表达式')
    return
  }
  if (!previewValid.value) {
    ElMessage.error('cron 表达式无效：' + previewError.value)
    return
  }
  if (formData.value.ActionType === 'http' && !formData.value.URL.trim()) {
    ElMessage.error('HTTP 动作必须填写 URL')
    return
  }
  if (formData.value.ActionType === 'shell' && !formData.value.Command.trim()) {
    ElMessage.error('命令动作必须填写命令')
    return
  }

  saving.value = true
  const payload = {
    name: formData.value.Name.trim(),
    cronExpr: formData.value.CronExpr.trim(),
    actionType: formData.value.ActionType,
    actionConfig: buildActionConfig(),
    timeoutSec: Number(formData.value.TimeoutSec) || 30,
    enabled: formData.value.Enabled,
  }
  try {
    if (editingId.value) {
      await sendMessage(`cronJobs/${editingId.value}`, 'PATCH', payload)
      ElMessage.success('已保存')
    } else {
      await sendMessage('cronJobs', 'POST', payload)
      ElMessage.success('已创建')
    }
    showDialog.value = false
    await fetchJobs()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    saving.value = false
  }
}

async function toggleJob(row: CronJob) {
  try {
    await sendMessage(`cronJobs/${row.ID}/toggle`, 'PATCH', {})
    await fetchJobs()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    await fetchJobs()
  }
}

async function deleteJob(row: CronJob) {
  try {
    await ElMessageBox.confirm(`确认删除定时任务「${row.Name}」？删除后不再调度。`, '警告', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'error',
    })
  } catch {
    return
  }
  try {
    await sendMessage(`cronJobs/${row.ID}`, 'DELETE')
    ElMessage.success('已删除')
    await fetchJobs()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

async function runJob(row: CronJob) {
  try {
    const run: any = await sendMessage(`cronJobs/${row.ID}/run`, 'POST', {})
    const ok = run && run.Success
    if (ok) ElMessage.success('执行成功')
    else ElMessage.warning('执行失败，请查看运行历史')
    await fetchJobs()
  } catch (e: any) {
    ElMessage.error(e?.message || '触发失败')
  }
}

const showHistory = ref(false)
const historyJobId = ref<number | null>(null)
const historyJobName = ref('')
const runs = ref<CronJobRun[]>([])
const runsLoading = ref(false)

async function openHistory(row: CronJob) {
  historyJobId.value = row.ID
  historyJobName.value = row.Name
  showHistory.value = true
  runsLoading.value = true
  try {
    const res: any = await sendMessage(`cronJobs/${row.ID}/runs`, 'GET', {page: 1, size: 50})
    runs.value = res?.items ?? []
  } catch (e: any) {
    ElMessage.error(e?.message || '加载历史失败')
    runs.value = []
  } finally {
    runsLoading.value = false
  }
}

onMounted(fetchJobs)
</script>

<template>
  <div>
    <div class="section-toolbar flex items-center justify-between">
      <span class="text-primary text-base section-title">定时任务</span>
      <div class="section-actions">
        <el-button size="small" type="primary" @click="openCreate">+ 新建定时任务</el-button>
      </div>
    </div>

    <el-table :data="jobs" v-loading="loading" border stripe class="task-table">
      <el-table-column label="名称" width="140" prop="Name"/>
      <el-table-column label="Cron 表达式" width="140" prop="CronExpr"/>
      <el-table-column label="动作" min-width="140">
        <template #default="{ row }">
          <el-tag size="small" :type="actionTypeMap[row.ActionType]?.type || 'info'">
            {{ actionTypeMap[row.ActionType]?.text || row.ActionType }}
          </el-tag>
          <span v-if="row.ActionType === 'http'" class="ml-sm text-secondary text-xs break-all">{{ row.ActionConfig }}</span>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.Enabled" @change="toggleJob(row)"/>
        </template>
      </el-table-column>
      <el-table-column label="上次运行" width="200">
        <template #default="{ row }">{{ fmtTime(row.LastRunAt) }}</template>
      </el-table-column>
      <el-table-column label="上次状态" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="statusMap[row.LastStatus]?.type || 'info'">
            {{ statusMap[row.LastStatus]?.text || '—' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="下次执行" width="200">
        <template #default="{ row }">
          <span v-if="row.Enabled">{{ fmtTime(row.NextRun) }}</span>
          <span v-else class="text-secondary">已停用</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <div class="op-actions">
            <el-button size="small" type="primary" text @click="runJob(row)">运行</el-button>
            <el-button size="small" text @click="openHistory(row)">历史</el-button>
            <el-button size="small" text @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" text @click="deleteJob(row)">删除</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showDialog" :title="editingId ? '编辑定时任务' : '新建定时任务'" width="34rem">
      <el-form label-position="top">
        <el-form-item label="任务名称">
          <el-input v-model="formData.Name" placeholder="请输入任务名称"/>
        </el-form-item>
        <el-form-item label="Cron 表达式">
          <el-input v-model="formData.CronExpr" placeholder="如: */5 * * * * (每5分钟)"/>
          <div class="cron-preview mt-xs">
            <span v-if="!formData.CronExpr.trim()" class="text-secondary text-xs">输入表达式后实时校验并预览下次执行时间</span>
            <template v-else>
              <span v-if="previewValid" class="text-success text-xs">✓ 表达式有效</span>
              <span v-else class="text-danger text-xs">✗ {{ previewError }}</span>
              <div v-if="previewValid && previewRuns.length" class="text-xs text-secondary mt-xs">
                未来 5 次：<br/>
                <div v-for="(r, i) in previewRuns" :key="i">{{ i + 1 }}. {{ r }}</div>
              </div>
            </template>
          </div>
        </el-form-item>
        <el-form-item label="动作类型">
          <el-select v-model="formData.ActionType">
            <el-option label="HTTP 请求 / Webhook" value="http"/>
            <el-option label="执行命令 / 脚本（预留）" value="shell"/>
          </el-select>
        </el-form-item>

        <el-alert v-if="formData.ActionType === 'shell'" type="warning" :closable="false" show-icon
                  class="mb-sm" title="命令执行功能默认关闭，仅当后端 FEATURE_CRON_SHELL=true 时才会真正执行；否则运行会记录「未启用」。"/>

        <template v-if="formData.ActionType === 'http'">
          <div class="form-row">
            <el-form-item label="方法">
              <el-select v-model="formData.Method" style="width: 100%">
                <el-option label="GET" value="GET"/>
                <el-option label="POST" value="POST"/>
                <el-option label="PUT" value="PUT"/>
                <el-option label="DELETE" value="DELETE"/>
                <el-option label="PATCH" value="PATCH"/>
              </el-select>
            </el-form-item>
            <el-form-item label="超时(秒)">
              <el-input-number v-model="formData.TimeoutSec" :min="1" :max="600" controls-position="right" style="width: 100%"/>
            </el-form-item>
          </div>
          <el-form-item label="URL（以 / 开头表示调用本机同名接口）">
            <el-input v-model="formData.URL" placeholder="https://example.com/hook 或 /api/systemJobs/sweep"/>
          </el-form-item>
          <el-form-item label="请求头（JSON 或每行 Key: Value）">
            <el-input v-model="formData.Headers" type="textarea" :rows="3" placeholder='{"Authorization":"Bearer x"}'/>
          </el-form-item>
          <el-form-item label="请求体">
            <el-input v-model="formData.Body" type="textarea" :rows="3" placeholder="可选"/>
          </el-form-item>
        </template>

        <template v-else>
          <el-form-item label="命令">
            <el-input v-model="formData.Command" placeholder="如: echo hello 或 C:\\scripts\\backup.bat"/>
          </el-form-item>
          <el-form-item label="工作目录（可选）">
            <el-input v-model="formData.WorkDir" placeholder="可选"/>
          </el-form-item>
        </template>

        <el-form-item label="启用">
          <el-switch v-model="formData.Enabled"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showHistory" :title="`运行历史 — ${historyJobName}`" width="40rem">
      <el-table :data="runs" v-loading="runsLoading" border stripe max-height="50vh">
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
.task-table {
  width: 100%;
}

.form-row {
  display: flex;
  gap: var(--space-md);
}

.form-row .el-form-item {
  flex: 1;
  margin-bottom: 18px;
}

.cron-preview {
  line-height: 1.6;
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

.op-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
</style>
