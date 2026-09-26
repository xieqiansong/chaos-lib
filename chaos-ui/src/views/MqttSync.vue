<script setup lang="ts">
// MQTT 同步界面：状态面板 + 配置驱动 DataTable（内置查看/编辑/删除）。
// 新建改为平铺广播表单（#toolbar 插槽 + hide-create），广播并记录到本地消息表；
// 状态查询、主题级删除为扩展能力，由本页自行调用 mqttSyncApi 处理。
import {onMounted, onUnmounted, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import DataTable from '@/components/DataTable.vue'
import {CRUD_ACTION, type DataTableColumn, type FormField} from '@/components/dataTable/types'
import {mqttSyncApi, type MqttStatus} from '@/api/mqttSync'

const status = ref<MqttStatus | null>(null)
// 平铺广播表单：主题同时作为「广播」与「删除该主题消息」的目标
const channel = ref('broadcast')
const payload = ref('')
const sending = ref(false)
const tableRef = ref<InstanceType<typeof DataTable> | null>(null)
let timer: number | undefined

const columns: DataTableColumn[] = [
  {field: 'Channel', title: '主题', minWidth: 140, searchable: true},
  {field: 'NodeID', title: '节点', width: 170, searchable: true},
  {field: 'IsSelf', title: '来源', width: 170, formatter: (row: any) => (row.IsSelf ? '本机' : '对端'),},
  {field: 'Payload', title: '内容', minWidth: 280, showOverflowTooltip: true},
  {field: 'CreatedAt', title: '时间', width: 170, type: 'datetime'},
  {field: '__actions', title: '操作', width: 140, type: 'actions', fixed: 'right'},
]

// 表单字段配置：驱动 DataTable 内置「查看 / 编辑」弹窗（新建走上方平铺表单）。
// 缺省主题为 broadcast，内容必填。
const fields: FormField[] = [
  {field: 'Channel', title: '主题', type: 'text', span: 12, required: true, placeholder: '如 broadcast'},
  {field: 'Payload', title: '内容', type: 'textarea', rows: 4, span: 24, required: true, placeholder: '消息内容，将广播到集群'},
]

async function refreshStatus() {
  try {
    status.value = await mqttSyncApi.status()
  } catch {
    status.value = null
  }
}

onMounted(() => {
  refreshStatus()
  timer = window.setInterval(refreshStatus, 5000)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})

// 广播：新建消息（AfterCreate 钩子广播到集群并记录到本地消息表）。
// 对应后端标准 CRUD 的 create，即 POST /api/mqttSync。
async function broadcast() {
  const body = payload.value.trim()
  if (!body) {
    ElMessage.warning('请输入内容')
    return
  }
  sending.value = true
  try {
    await mqttSyncApi.create({Channel: channel.value.trim() || 'broadcast', Payload: body})
    ElMessage.success('已广播并记录到本地消息表')
    payload.value = ''
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '广播失败')
  } finally {
    sending.value = false
  }
}

// 按主题批量删除：目标主题取平铺表单的主题输入。
async function removeChannel() {
  const name = channel.value.trim()
  if (!name) {
    ElMessage.warning('请输入要删除的主题')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除主题「${name}」下的全部消息？（仅标记删除，仍保留在库中）`,
      '删除确认',
      {type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消'},
    )
  } catch {
    return
  }
  try {
    await mqttSyncApi.deleteChannel(name)
    ElMessage.success(`已删除主题「${name}」的消息`)
    tableRef.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
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
    </div>

    <el-alert
        v-if="status?.enabled && !status?.connected"
        class="mb-sm"
        type="warning"
        show-icon
        :closable="false"
        title="MQTT 未连接：广播消息仅本地落库，无法送达其它节点"
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

    <DataTable
        ref="tableRef"
        :columns="columns"
        :api="mqttSyncApi"
        :fields="fields"
        title="消息"
        row-key="ID"
        :enabled-actions="[CRUD_ACTION.VIEW, CRUD_ACTION.EDIT, CRUD_ACTION.DELETE]"
        :default-sort="{field: 'created_at', order: 'descending'}"
    >
      <!-- 平铺广播表单：新建即广播并记录到本地消息表；主题同时作为「删除该主题消息」的目标 -->
      <template #toolbar>
        <div class="broadcast-form">
          <el-input
              v-model="channel"
              placeholder="主题（默认 broadcast）"
              size="small"
              clearable
              class="broadcast-channel"
          />
          <el-input
              v-model="payload"
              placeholder="内容，广播到集群并记录到本地消息表"
              size="small"
              clearable
              class="broadcast-payload"
              @keyup.enter="broadcast"
          />
          <el-button type="primary" size="small" :loading="sending" @click="broadcast">广播</el-button>
          <el-button type="danger" size="small" plain @click="removeChannel">删除该主题消息</el-button>
        </div>
      </template>

      <template #Payload="{ row }">
        <span class="payload-cell">{{ row.Payload }}</span>
      </template>
    </DataTable>
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

code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: var(--el-font-size-small);
}

.mb-sm {
  margin-bottom: var(--space-sm);
}

.broadcast-form {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  width: 100%;
}

.broadcast-channel {
  width: 200px;
  flex: 0 0 auto;
}

.broadcast-payload {
  flex: 1 1 auto;
  min-width: 200px;
}

.payload-cell {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-all;
}
</style>
