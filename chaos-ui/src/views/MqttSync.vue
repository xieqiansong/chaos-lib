<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {Refresh} from '@element-plus/icons-vue'
import {format, parseISO} from 'date-fns'
import {deleteMqttMessagesByChannel, getMqttMessages, getMqttStatus, type MqttMessage, type MqttStatus, sendMqttMessage,} from '@/utils/api'

const status = ref<MqttStatus | null>(null)
const messages = ref<MqttMessage[]>([])
const payload = ref('')
const channel = ref('')
const refreshing = ref(false)
const sending = ref(false)
const detailVisible = ref(false)
const detailTitle = ref('')
const detailPayload = ref('')
let timer: number | undefined

function showDetail(row: MqttMessage) {
  detailTitle.value = row.node_id
  detailPayload.value = row.payload
  detailVisible.value = true
}

// 尽量把 payload 美化为可读 JSON，失败则原样展示
function prettyPayload(v: string): string {
  try {
    return JSON.stringify(JSON.parse(v), null, 2)
  } catch {
    return v
  }
}

// copyPayload 复制文本到剪贴板，优先 navigator.clipboard（需安全上下文），失败降级到 textarea 方案
async function copyPayload(text: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      if (!ok) throw new Error('execCommand copy failed')
    }
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}

async function refreshStatus() {
  try {
    status.value = await getMqttStatus()
  } catch {
    status.value = null
  }
}

async function refreshMessages() {
  try {
    const list = await getMqttMessages()
    // 按 topic（channel）升序展示，便于同类消息归拢查看
    messages.value = [...list].sort((a, b) => a.channel.localeCompare(b.channel, 'zh-CN'))
  } catch {
    /* 轮询出错忽略，下次重试 */
  }
}

async function refresh() {
  refreshing.value = true
  try {
    await Promise.all([refreshStatus(), refreshMessages()])
  } finally {
    refreshing.value = false
  }
}

async function send() {
  if (!payload.value.trim()) {
    ElMessage.warning('请输入消息内容')
    return
  }
  sending.value = true
  try {
    await sendMqttMessage(payload.value, channel.value.trim() || undefined)
    payload.value = ''
    ElMessage.success('已发送')
    await refreshMessages()
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error('发送失败：' + msg)
  } finally {
    sending.value = false
  }
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 3000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

async function removeByChannel(ch: string) {
  try {
    await deleteMqttMessagesByChannel(ch)
    ElMessage.success(`已删除 topic「${ch}」的全部消息`)
    await refreshMessages()
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error('删除失败：' + msg)
  }
}

function confirmRemove(row: MqttMessage) {
  // 二次确认：按 topic 软删除数据库中的全部消息
  ElMessageBox.confirm(
      `确认删除 topic「${row.channel}」下的全部消息？（仅标记删除，仍保留在库中）`,
      '删除确认',
      {type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消'},
  )
      .then(() => removeByChannel(row.channel))
      .catch(() => {
      })
}

function fmtTime(v: string): string {
  if (!v) return '-'
  try {
    return format(parseISO(v), 'yyyy-MM-dd HH:mm:ss')
  } catch {
    return v
  }
}
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">MQTT 同步</span>
      <div class="metric-strip">
        <div class="metric-chip">
          <span class="metric-label">启用</span>
          <el-tag :type="status?.enabled ? 'success' : 'info'" size="small">
            {{ status?.enabled ? '是' : '否' }}
          </el-tag>
        </div>
        <div class="metric-chip">
          <span class="metric-label">连接</span>
          <el-tag :type="status?.connected ? 'success' : 'warning'" size="small">
            {{ status?.connected ? '已连接' : '离线' }}
          </el-tag>
        </div>
        <div class="metric-chip">
          <span class="metric-label">节点</span>
          <code>{{ status?.node_id || '-' }}</code>
        </div>
        <div class="metric-chip">
          <span class="metric-label">加密</span>
          <el-tag :type="status?.encrypt ? 'success' : 'warning'" size="small">
            {{ status?.encrypt ? '已加密' : '明文' }}
          </el-tag>
        </div>
        <div class="metric-chip">
          <span class="metric-label">Broker</span>
          <code>{{ status?.broker || '-' }}</code>
        </div>
        <div class="metric-chip">
          <span class="metric-label">前缀</span>
          <code>{{ status?.prefix || '-' }}</code>
        </div>
      </div>
      <div class="section-actions">
        <el-button size="small" :icon="Refresh" :loading="refreshing" @click="refresh">刷新</el-button>
      </div>
    </div>

    <el-alert
        v-if="status?.enabled && !status?.connected"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="MQTT 未连接：消息仅本地落库，无法广播给其它节点"
    />
    <el-alert
        v-if="status?.enabled && !status?.encrypt"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="已启用但未加密：MQTT_ENCRYPT_KEY 缺失或无效，消息以明文广播"
    />
    <el-alert
        v-if="!status?.enabled"
        class="mb-sm"
        type="info"
        show-icon
        :closable="false"
        title="MQTT 同步未启用：请在 .env 设置 MQTT_ENABLED=true 并重启"
    />

    <div class="send-row">
      <el-input v-model="channel" placeholder="topic（缺省 broadcast）" class="channel-input"/>
      <el-input
          v-model="payload"
          placeholder="输入消息内容，回车发送"
          class="payload-input"
          @keyup.enter="send"
      />
      <el-button type="primary" :loading="sending" @click="send">发送</el-button>
    </div>

    <el-card shadow="never" class="table-card">
      <template #header>
        <div class="table-header">
          <span class="text-primary text-sm">消息（{{ messages.length }}）</span>
          <span class="text-secondary text-xs">每个 topic 仅展示最新一条，旧消息已存库</span>
        </div>
      </template>
      <el-skeleton v-if="refreshing && messages.length === 0" :rows="6" animated/>
      <el-empty v-else-if="messages.length === 0" description="暂无消息"/>
      <el-table v-else :data="messages" stripe empty-text="暂无消息" style="width: 100%">
        <el-table-column prop="channel" label="topic" width="300"/>
        <el-table-column label="内容" min-width="240" prop="payload" show-overflow-tooltip/>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="copyPayload(row.payload)">复制</el-button>
            <el-button size="small" type="primary" @click="showDetail(row)">详情</el-button>
            <el-button size="small" type="danger" @click="confirmRemove(row)">删除</el-button>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">
            {{ fmtTime(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" :title="`消息详情：${detailTitle}`" width="75%">
      <pre class="detail-pre">{{ prettyPayload(detailPayload) }}</pre>
      <template #footer>
        <el-button @click="copyPayload(detailPayload)">复制内容</el-button>
        <el-button type="primary" @click="detailVisible = false">关闭</el-button>
      </template>
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

.send-row {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-lg);
}

.channel-input {
  max-width: 220px;
}

.payload-input {
  flex: 1;
}

.table-card {
  --el-card-padding: 12px;
}

.table-header {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.mb-sm {
  margin-bottom: var(--space-sm);
}

code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: var(--el-font-size-small);
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
