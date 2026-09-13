<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getMqttMessages,
  getMqttStatus,
  sendMqttMessage,
  type MqttMessage,
  type MqttStatus,
} from '@/utils/api'

const status = ref<MqttStatus | null>(null)
const messages = ref<MqttMessage[]>([])
const payload = ref('')
const channel = ref('')
const loading = ref(false)
let timer: number | undefined

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
      </div>
      <div class="status-row sub">
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
        <el-input v-model="channel" placeholder="topic（缺省 broadcast）" class="channel-input" />
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
        <el-table-column prop="channel" label="topic" width="160" />
        <el-table-column prop="payload" label="内容" min-width="240" />
        <el-table-column label="来源" width="160">
          <template #default="{ row }">
            <el-tag v-if="row.is_self" size="small" type="success">本机</el-tag>
            <span v-else class="mono">{{ row.node_id }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" min-width="200" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.mqtt-container {
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
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
</style>
