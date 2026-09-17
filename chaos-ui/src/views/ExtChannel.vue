<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {ElMessage} from 'element-plus'
import {Refresh} from '@element-plus/icons-vue'
import {
  getExtResponses,
  getExtStatus,
  pushExtCommand,
  type ExtCommand,
  type ExtResponse,
  type ExtStatus,
} from '@/utils/api'

const status = ref<ExtStatus | null>(null)
const responses = ref<ExtResponse[]>([])
const refreshing = ref(false)
const sending = ref(false)

// 指令表单
const cmdType = ref<'ping' | 'openTab' | 'custom'>('ping')
const cmdUrl = ref('')
const cmdText = ref('')
const cmdRaw = ref('')

// 本端「已下发」记录（前端本地，用于对照回传）
interface SentItem {
  time: string
  command: string
  pushed: number
}
const sentLog = ref<SentItem[]>([])

// 回传详情弹窗
const detailVisible = ref(false)
const detailEcho = ref('')

let timer: number | undefined

function now(): string {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

// 美化 JSON，失败原样
function pretty(v: any): string {
  try {
    return JSON.stringify(typeof v === 'string' ? JSON.parse(v) : v, null, 2)
  } catch {
    return String(v ?? '')
  }
}

async function refreshStatus() {
  try {
    status.value = await getExtStatus()
  } catch {
    status.value = null
  }
}

async function refreshResponses() {
  try {
    responses.value = await getExtResponses()
  } catch {
    /* 轮询出错忽略，下次重试 */
  }
}

async function refresh() {
  refreshing.value = true
  try {
    await Promise.all([refreshStatus(), refreshResponses()])
  } finally {
    refreshing.value = false
  }
}

async function doSend(cmd: ExtCommand) {
  sending.value = true
  try {
    const res = await pushExtCommand(cmd)
    sentLog.value.unshift({time: now(), command: pretty(cmd), pushed: res.pushed ?? 0})
    if ((res.pushed ?? 0) > 0) {
      ElMessage.success(`已下发，送达 ${res.pushed} 个扩展`)
    } else {
      ElMessage.warning('已下发，但当前没有已连接的扩展（pushed=0）')
    }
    await refreshResponses()
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error('下发失败：' + msg)
  } finally {
    sending.value = false
  }
}

// 一键 Ping 测试（对应后端 handleCommand 的 ping → pong）
async function pingTest() {
  await doSend({type: 'ping', id: `ping-${Date.now()}`})
}

async function sendCommand() {
  if (cmdType.value === 'ping') {
    await doSend({type: 'ping', id: `ping-${Date.now()}`})
    return
  }
  if (cmdType.value === 'openTab') {
    if (!cmdUrl.value.trim()) {
      ElMessage.warning('openTab 指令需要填写 URL')
      return
    }
    await doSend({type: 'openTab', url: cmdUrl.value.trim(), id: `tab-${Date.now()}`})
    return
  }
  // custom：直接解析 JSON 作为指令体
  let parsed: ExtCommand
  try {
    parsed = JSON.parse(cmdRaw.value)
  } catch {
    ElMessage.error('自定义指令不是合法 JSON')
    return
  }
  if (!parsed.type) {
    ElMessage.error('自定义指令缺少 type 字段')
    return
  }
  await doSend(parsed)
}

function showEcho(row: ExtResponse) {
  detailEcho.value = pretty(row.echo ?? {})
  detailVisible.value = true
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 3000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">扩展通道</span>
      <div class="metric-strip">
        <div class="metric-chip">
          <span class="metric-label">已连接扩展</span>
          <el-tag :type="(status?.connected ?? 0) > 0 ? 'success' : 'warning'" size="small">
            {{ status?.connected ?? 0 }}
          </el-tag>
        </div>
      </div>
      <div class="section-actions">
        <el-button size="small" :icon="Refresh" :loading="refreshing" @click="refresh">刷新</el-button>
      </div>
    </div>

    <el-alert
        v-if="!(status?.connected ?? 0) > 0"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="未连接扩展：请确认浏览器插件已加载，并在插件 popup 中点击「连接后端」"
    />

    <el-card shadow="never" class="send-card">
      <template #header>
        <span class="text-primary text-sm">下发指令（后端 → 扩展）</span>
      </template>
      <div class="send-row">
        <el-select v-model="cmdType" class="type-select" placeholder="指令类型">
          <el-option label="Ping 测试" value="ping"/>
          <el-option label="打开标签页" value="openTab"/>
          <el-option label="自定义 JSON" value="custom"/>
        </el-select>
        <el-input
            v-if="cmdType === 'openTab'"
            v-model="cmdUrl"
            class="url-input"
            placeholder="URL，如 https://example.com"
            @keyup.enter="sendCommand"
        />
        <el-input
            v-if="cmdType === 'custom'"
            v-model="cmdRaw"
            class="raw-input"
            type="textarea"
            :rows="2"
            placeholder='自定义指令 JSON，如 {"type":"ping","id":"abc"}'
        />
        <el-input
            v-if="cmdType !== 'custom'"
            v-model="cmdText"
            class="text-input"
            placeholder="附带文本（可选）"
            @keyup.enter="sendCommand"
        />
        <el-button type="primary" :loading="sending" @click="sendCommand">下发</el-button>
        <el-button :loading="sending" @click="pingTest">Ping 测试</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="table-card">
      <template #header>
        <span class="text-primary text-sm">已下发指令（本机记录，{{ sentLog.length }}）</span>
      </template>
      <el-empty v-if="sentLog.length === 0" description="暂无下发记录"/>
      <el-table v-else :data="sentLog" stripe style="width: 100%">
        <el-table-column prop="time" label="时间" width="180"/>
        <el-table-column label="送达" width="80">
          <template #default="{ row }">
            <el-tag :type="row.pushed > 0 ? 'success' : 'info'" size="small">{{ row.pushed }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="指令" min-width="280" prop="command" show-overflow-tooltip/>
      </el-table>
    </el-card>

    <el-card shadow="never" class="table-card">
      <template #header>
        <span class="text-primary text-sm">扩展回传（交换记录，{{ responses.length }}）</span>
      </template>
      <el-empty v-if="responses.length === 0" description="暂无回传"/>
      <el-table v-else :data="responses" stripe style="width: 100%">
        <el-table-column prop="receivedAt" label="接收时间" width="180"/>
        <el-table-column label="类型" width="120" prop="type"/>
        <el-table-column label="结果" width="90">
          <template #default="{ row }">
            <el-tag :type="row.ok ? 'success' : 'danger'" size="small">{{ row.ok ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="id" label="指令ID" width="200" show-overflow-tooltip/>
        <el-table-column prop="error" label="错误" min-width="160" show-overflow-tooltip/>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" text @click="showEcho(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="回传详情（echo）" width="60%">
      <pre class="detail-pre">{{ detailEcho }}</pre>
    </el-dialog>
  </div>
</template>

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

.send-card {
  --el-card-padding: 12px;
  margin-bottom: var(--space-lg);
}

.send-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  align-items: center;
}

.type-select {
  width: 150px;
}

.url-input {
  flex: 1;
  min-width: 220px;
}

.text-input {
  width: 220px;
}

.raw-input {
  flex: 1;
  min-width: 280px;
}

.table-card {
  --el-card-padding: 12px;
  margin-bottom: var(--space-lg);
}

.mb-sm {
  margin-bottom: var(--space-sm);
}

.detail-pre {
  margin: 0;
  max-height: 60vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: var(--el-font-size-small);
  line-height: 1.5;
}
</style>
