<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {ElMessage} from 'element-plus'
import {format, parseISO} from 'date-fns'
import {getMqttMessages, getMqttStatus, type MqttMessage, type MqttStatus, sendMqttMessage,} from '@/utils/api'

const status = ref<MqttStatus | null>(null)
const messages = ref<MqttMessage[]>([])
const payload = ref('')
const channel = ref('')
const loading = ref(false)
const detailVisible = ref(false)
const detailPayload = ref('')
let timer: number | undefined

function showDetail(payload: string) {
  detailPayload.value = payload
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
    messages.value = await getMqttMessages()
  } catch {
    /* 轮询出错忽略，下次重试 */
  }
}

async function send() {
  if (!payload.value.trim()) {
    ElMessage.warning('请输入消息内容')
    return
  }
  loading.value = true
  try {
    await sendMqttMessage(payload.value, channel.value.trim() || undefined)
    payload.value = ''
    ElMessage.success('已发送')
    await refreshMessages()
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error('发送失败：' + msg)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refreshStatus()
  refreshMessages()
  timer = window.setInterval(() => {
    refreshStatus()
    refreshMessages()
  }, 3000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

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
  <div class="mqtt-container">
    <el-card shadow="hover" class="status-card">
      <div class="status-row">
        <span>启用：</span>
        <el-tag :type="status?.enabled ? 'success' : 'info'">{{ status?.enabled ? '是' : '否' }}</el-tag>
        <span class="label">连接：</span>
        <el-tag :type="status?.connected ? 'success' : 'warning'">
          {{ status?.connected ? '已连接' : '离线' }}
        </el-tag>
        <span class="label">节点：</span><code>{{ status?.node_id || '-' }}</code>
        <span class="label">加密：</span>
        <el-tag :type="status?.encrypt ? 'success' : 'warning'">
          {{ status?.encrypt ? '已加密' : '明文' }}
        </el-tag>
        <span class="label">Broker：</span><code>{{ status?.broker || '-' }}</code>
        <span class="label">前缀：</span><code>{{ status?.prefix || '-' }}</code>
      </div>
      <el-alert
          v-if="status?.enabled && !status?.connected"
          class="alert"
          type="warning"
          :closable="false"
          title="MQTT 未连接：消息仅本地落库，无法广播给其它节点"
      />
      <el-alert
          v-if="status?.enabled && !status?.encrypt"
          class="alert"
          type="warning"
          :closable="false"
          title="已启用但未加密：MQTT_ENCRYPT_KEY 缺失或无效，消息以明文广播"
      />
      <el-alert
          v-if="!status?.enabled"
          class="alert"
          type="info"
          :closable="false"
          title="MQTT 同步未启用：请在 .env 设置 MQTT_ENABLED=true 并重启"
      />
    </el-card>

    <el-card shadow="hover" class="send-card">
      <div class="send-row">
        <el-input v-model="channel" placeholder="topic（缺省 broadcast）" class="channel-input"/>
        <el-input
            v-model="payload"
            placeholder="输入消息内容，回车发送"
            class="payload-input"
            @keyup.enter="send"
        />
        <el-button type="primary" :loading="loading" @click="send">发送</el-button>
      </div>
    </el-card>

    <el-card shadow="hover">
      <template #header>消息（每个 topic 仅展示最新一条，旧消息已存库）</template>
      <el-table :data="messages" empty-text="暂无消息" style="width: 100%">
        <el-table-column prop="channel" label="topic" width="160"/>
        <el-table-column label="内容" min-width="240">
          <template #default="{ row }">
            <span class="payload-cell">{{ row.payload }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="copyPayload(row.payload)">复制</el-button>
            <el-button size="small" @click="showDetail(row.payload)">详情</el-button>
          </template>
        </el-table-column>
        <el-table-column label="来源" width="160">
          <template #default="{ row }">
            <el-tag v-if="row.is_self" size="small" type="success">本机</el-tag>
            <span v-else class="mono">{{ row.node_id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="160">
          <template #default="{ row }">
            {{ fmtTime(row.created_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="detailVisible" title="消息详情" width="75%">
      <pre class="detail-pre">{{ prettyPayload(detailPayload) }}</pre>
    </el-dialog>
  </div>
</template>

<style scoped>
.mqtt-container {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.status-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.status-row.sub {
  margin-top: 0.5rem;
  color: var(--el-text-color-secondary);
}

.label {
  margin-left: 1rem;
}

.mono {
  font-family: monospace;
  font-size: 12px;
}

.send-row {
  display: flex;
  gap: 0.5rem;
}

.channel-input {
  max-width: 220px;
}

.payload-input {
  flex: 1;
}

.alert {
  margin-top: 0.75rem;
}

code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}
.detail-pre {
  margin: 0;
  max-height: 60vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.5;
}
.payload-cell {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
