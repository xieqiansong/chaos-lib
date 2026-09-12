<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import {
  createPortForward,
  createSshConn,
  deletePortForward,
  deleteSshConn,
  getPortForwards,
  getSshConns,
  testSshConn,
  updatePortForward,
  updatePortForwardStatus,
  updateSshConn,
  type PortForward,
  type PortForwardDirection,
  type SshAuthType,
  type SshConnection,
  type SshConnectionPayload,
} from '@/utils/api'

const props = defineProps<{
  searchText: string
}>()

const loading = ref(false)
const error = ref('')

const sshConns = ref<SshConnection[]>([])
const forwards = ref<PortForward[]>([])

const authTypeText: Record<string, string> = {password: '密码', key: '私钥'}

const connNameById = computed(() => {
  const map: Record<number, string> = {}
  for (const conn of sshConns.value) map[conn.Id] = conn.Name
  return map
})

function matches(text: string | undefined, keyword: string): boolean {
  return (text || '').toLowerCase().includes(keyword)
}

const filteredConns = computed(() => {
  const keyword = (props.searchText || '').trim().toLowerCase()
  if (!keyword) return sshConns.value
  return sshConns.value.filter(conn =>
      matches(conn.Name, keyword) || matches(conn.Host, keyword) ||
      matches(conn.Username, keyword) || matches(conn.Remark, keyword))
})

const filteredForwards = computed(() => {
  const keyword = (props.searchText || '').trim().toLowerCase()
  if (!keyword) return forwards.value
  return forwards.value.filter(rule =>
      matches(rule.Name, keyword) || matches(rule.TargetHost, keyword) ||
      matches(rule.Remark, keyword))
})

function pickErrorMessage(e: unknown, fallback: string): string {
  const raw = e instanceof Error ? e.message : String(e)
  const idx = raw.indexOf('{')
  if (idx >= 0) {
    try {
      const parsed = JSON.parse(raw.slice(idx))
      if (parsed && typeof parsed.error === 'string' && parsed.error) return parsed.error
    } catch {
      /* 非 JSON 响应，回退到默认提示 */
    }
  }
  return fallback
}

async function fetchAll() {
  loading.value = true
  error.value = ''
  try {
    const [conns, rules] = await Promise.all([getSshConns(), getPortForwards()])
    sshConns.value = conns || []
    forwards.value = rules || []
  } catch (e) {
    error.value = '获取端口转发数据失败，请确认后端服务已启动'
    console.error(e)
  } finally {
    loading.value = false
  }
}

// ── SSH 连接 ────────────────────────────────────────────────────────

interface ConnForm {
  Id: number
  Name: string
  Host: string
  Port: number
  Username: string
  AuthType: SshAuthType
  Password: string
  PrivateKey: string
  Passphrase: string
  Remark: string
}

function emptyConnForm(): ConnForm {
  return {
    Id: 0, Name: '', Host: '', Port: 22, Username: '',
    AuthType: 'password', Password: '', PrivateKey: '', Passphrase: '', Remark: '',
  }
}

const showConnModal = ref(false)
const connForm = ref<ConnForm>(emptyConnForm())
const isEditConn = computed(() => connForm.value.Id > 0)
const testingId = ref(0)

function openCreateConn() {
  connForm.value = emptyConnForm()
  showConnModal.value = true
}

function openEditConn(conn: SshConnection) {
  connForm.value = {
    Id: conn.Id, Name: conn.Name, Host: conn.Host, Port: conn.Port,
    Username: conn.Username, AuthType: conn.AuthType,
    Password: '', PrivateKey: '', Passphrase: '', Remark: conn.Remark,
  }
  showConnModal.value = true
}

async function submitConn() {
  const form = connForm.value
  if (!form.Name.trim() || !form.Host.trim() || !form.Username.trim()) {
    ElMessage.warning('名称、SSH 主机、用户名不能为空')
    return
  }
  if (!form.Id) {
    if (form.AuthType === 'password' && !form.Password) {
      ElMessage.warning('请填写密码')
      return
    }
    if (form.AuthType === 'key' && !form.PrivateKey.trim()) {
      ElMessage.warning('请粘贴私钥内容')
      return
    }
  }
  const payload: SshConnectionPayload = {
    Name: form.Name.trim(), Host: form.Host.trim(), Port: form.Port,
    Username: form.Username.trim(), AuthType: form.AuthType, Remark: form.Remark,
  }
  // 凭据留空表示保持原值，后端不回显
  if (form.Password) payload.Password = form.Password
  if (form.PrivateKey.trim()) payload.PrivateKey = form.PrivateKey
  if (form.Passphrase) payload.Passphrase = form.Passphrase

  try {
    if (form.Id) {
      await updateSshConn(form.Id, payload)
      ElMessage.success('连接已保存')
    } else {
      await createSshConn(payload)
      ElMessage.success('连接已创建')
    }
    showConnModal.value = false
    await fetchAll()
  } catch (e) {
    ElMessage.error(pickErrorMessage(e, '保存失败'))
  }
}

async function testConn(conn: SshConnection) {
  testingId.value = conn.Id
  try {
    const res = await testSshConn(conn.Id)
    const version = res?.data?.serverVersion ? ` (${res.data.serverVersion})` : ''
    ElMessage.success(`连接成功${version}`)
  } catch (e) {
    ElMessage.error(pickErrorMessage(e, '连接失败'))
  } finally {
    testingId.value = 0
  }
}

async function removeConn(conn: SshConnection) {
  try {
    await ElMessageBox.confirm(`确认删除 SSH 连接「${conn.Name}」？`, '警告', {
      type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await deleteSshConn(conn.Id)
    ElMessage.success('连接已删除')
    await fetchAll()
  } catch (e) {
    ElMessage.error(pickErrorMessage(e, '删除失败'))
  }
}

// ── 转发规则 ────────────────────────────────────────────────────────

interface ForwardForm {
  Id: number
  Name: string
  Direction: PortForwardDirection
  Port: number
  BindAddress: string
  TargetHost: string
  TargetPort: number
  SshConnectionId: number
  Remark: string
}

function emptyForwardForm(): ForwardForm {
  return {
    Id: 0, Name: '', Direction: 'local', Port: 0, BindAddress: '',
    TargetHost: '127.0.0.1', TargetPort: 0,
    SshConnectionId: sshConns.value[0]?.Id || 0, Remark: '',
  }
}

const showForwardModal = ref(false)
const forwardForm = ref<ForwardForm>(emptyForwardForm())
const isEditForward = computed(() => forwardForm.value.Id > 0)
const isRemoteForm = computed(() => forwardForm.value.Direction === 'remote')

/** 远程转发绑定非回环地址时的风险提示 */
const bindAddressRisk = computed(() => {
  if (!isRemoteForm.value) return ''
  const addr = forwardForm.value.BindAddress.trim()
  if (!addr || addr === '127.0.0.1' || addr === '::1') return ''
  return '监听地址非回环：SSH 服务器需开启 GatewayPorts，且会把本机服务暴露给服务器网络'
})

function onDirectionChange() {
  // 切换方向后监听地址含义变化，清空以回落到该方向的默认值
  forwardForm.value.BindAddress = ''
}

function openCreateForward() {
  if (sshConns.value.length === 0) {
    ElMessage.warning('请先创建一条 SSH 连接')
    return
  }
  forwardForm.value = emptyForwardForm()
  showForwardModal.value = true
}

function openEditForward(rule: PortForward) {
  forwardForm.value = {
    Id: rule.Id, Name: rule.Name,
    Direction: rule.Direction === 'remote' ? 'remote' : 'local',
    Port: rule.Port, BindAddress: rule.BindAddress || '',
    TargetHost: rule.TargetHost,
    TargetPort: rule.TargetPort, SshConnectionId: rule.SshConnectionId, Remark: rule.Remark,
  }
  showForwardModal.value = true
}

async function submitForward() {
  const form = forwardForm.value
  const portLabel = form.Direction === 'remote' ? '远端监听端口' : '本地监听端口'
  if (!form.SshConnectionId) {
    ElMessage.warning('请选择 SSH 连接')
    return
  }
  if (!form.Port || form.Port < 1 || form.Port > 65535) {
    ElMessage.warning(`${portLabel}需在 1-65535 之间`)
    return
  }
  if (!form.TargetHost.trim()) {
    ElMessage.warning('目标主机不能为空')
    return
  }
  if (!form.TargetPort || form.TargetPort < 1 || form.TargetPort > 65535) {
    ElMessage.warning('目标端口需在 1-65535 之间')
    return
  }
  const payload = {
    Name: form.Name.trim(), Direction: form.Direction, Port: form.Port,
    BindAddress: form.BindAddress.trim(), TargetHost: form.TargetHost.trim(),
    TargetPort: form.TargetPort, SshConnectionId: form.SshConnectionId, Remark: form.Remark,
  }
  try {
    if (form.Id) {
      await updatePortForward(form.Id, payload)
      ElMessage.success('规则已保存')
    } else {
      await createPortForward(payload)
      ElMessage.success('规则已创建')
    }
    showForwardModal.value = false
    await fetchAll()
  } catch (e) {
    ElMessage.error(pickErrorMessage(e, '保存失败'))
  }
}

async function toggleForward(rule: PortForward, next: boolean) {
  const previous = rule.Status
  rule.Status = next
  try {
    await updatePortForwardStatus(rule.Id, next)
    await fetchAll()
  } catch (e) {
    rule.Status = previous
    ElMessage.error(pickErrorMessage(e, next ? '启动失败' : '停止失败'))
  }
}

async function removeForward(rule: PortForward) {
  try {
    await ElMessageBox.confirm(`确认删除转发规则「${rule.Name}」？`, '警告', {
      type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await deletePortForward(rule.Id)
    ElMessage.success('规则已删除')
    await fetchAll()
  } catch (e) {
    ElMessage.error(pickErrorMessage(e, '删除失败'))
  }
}

onMounted(() => {
  fetchAll()
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">SSH 连接管理</span>
      <span class="text-secondary text-xs">保存 SSH 主机与凭据（密码或私钥），供转发规则复用</span>
      <div class="section-actions">
        <el-button size="small" type="primary" @click="openCreateConn">+ 新建连接</el-button>
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

    <el-skeleton v-if="loading" :rows="3" animated/>

    <div v-else-if="filteredConns.length === 0" class="empty-wrap">
      <el-empty description="暂无 SSH 连接"/>
    </div>

    <el-table v-else :data="filteredConns" class="portfwd-table">
      <el-table-column prop="Name" label="名称" min-width="120"/>
      <el-table-column label="主机" min-width="180">
        <template #default="{row}">
          <span class="font-mono">{{ row.Username }}@{{ row.Host }}:{{ row.Port }}</span>
        </template>
      </el-table-column>
      <el-table-column label="认证方式" width="100">
        <template #default="{row}">
          <el-tag type="info">{{ authTypeText[row.AuthType] || row.AuthType }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="凭据" width="90">
        <template #default="{row}">
          <span class="text-secondary text-xs">
            {{ row.HasPassword || row.HasPrivateKey ? '已配置' : '未配置' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="Remark" label="备注" min-width="120"/>
      <el-table-column label="操作" width="220">
        <template #default="{row}">
          <el-button
              size="small"
              :loading="testingId === row.Id"
              @click="testConn(row)">
            测试
          </el-button>
          <el-button size="small" type="primary" @click="openEditConn(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="removeConn(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="section-toolbar mt-lg">
      <span class="text-primary text-base section-title">转发规则管理</span>
      <span class="text-secondary text-xs">本地转发（ssh -L）/ 远程转发（ssh -R）双向 SSH 隧道</span>
      <div class="section-actions">
        <el-button size="small" type="primary" @click="openCreateForward">+ 新建规则</el-button>
      </div>
    </div>

    <el-skeleton v-if="loading" :rows="3" animated/>

    <div v-else-if="filteredForwards.length === 0" class="empty-wrap">
      <el-empty description="暂无转发规则"/>
    </div>

    <el-table v-else :data="filteredForwards" class="portfwd-table">
      <el-table-column prop="Name" label="名称" min-width="140"/>
      <el-table-column label="方向" width="100">
        <template #default="{row}">
          <el-tag :type="row.Direction === 'remote' ? 'warning' : 'info'">
            {{ row.Direction === 'remote' ? '远程 -R' : '本地 -L' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="监听端口" width="140">
        <template #default="{row}">
          <div class="font-mono">{{ row.Port }}</div>
          <div class="text-secondary text-xs">
            {{ row.Direction === 'remote' ? '远端端口' : '本地端口' }} · {{ row.BindAddress }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="目标" min-width="200">
        <template #default="{row}">
          <span class="font-mono">{{ row.TargetHost }}:{{ row.TargetPort }}</span>
        </template>
      </el-table-column>
      <el-table-column label="SSH 连接" min-width="140">
        <template #default="{row}">
          {{ connNameById[row.SshConnectionId] || `#${row.SshConnectionId}` }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{row}">
          <el-tag :type="row.Status ? 'success' : 'info'">
            {{ row.Status ? '运行中' : '未运行' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近错误" min-width="160">
        <template #default="{row}">
          <span class="text-secondary text-xs">{{ row.LastError || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="启停" width="80">
        <template #default="{row}">
          <el-switch
              :model-value="row.Status"
              @update:model-value="(val: boolean) => toggleForward(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{row}">
          <el-button size="small" type="primary" @click="openEditForward(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="removeForward(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showConnModal" :title="isEditConn ? '编辑 SSH 连接' : '新建 SSH 连接'" width="620px">
      <el-form :model="connForm" label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="connForm.Name" placeholder="如：跳板机 / 生产服务器"/>
        </el-form-item>
        <el-form-item label="SSH 主机">
          <el-input v-model="connForm.Host" placeholder="如：192.168.1.10 或 example.com"/>
        </el-form-item>
        <el-form-item label="SSH 端口">
          <el-input-number v-model="connForm.Port" :min="1" :max="65535" controls-position="right"/>
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="connForm.Username" placeholder="如：root"/>
        </el-form-item>
        <el-form-item label="认证方式">
          <el-radio-group v-model="connForm.AuthType">
            <el-radio-button value="password">密码</el-radio-button>
            <el-radio-button value="key">私钥</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="connForm.AuthType === 'password'" label="密码">
          <el-input
              v-model="connForm.Password"
              type="password"
              show-password
              :placeholder="isEditConn ? '留空表示不修改' : '请输入 SSH 密码'"/>
        </el-form-item>
        <template v-else>
          <el-form-item label="私钥">
            <el-input
                v-model="connForm.PrivateKey"
                type="textarea"
                :rows="6"
                :placeholder="isEditConn ? '留空表示不修改私钥' : '粘贴 PEM 私钥内容（-----BEGIN ... PRIVATE KEY-----）'"/>
          </el-form-item>
          <el-form-item label="私钥口令">
            <el-input
                v-model="connForm.Passphrase"
                type="password"
                show-password
                :placeholder="isEditConn ? '留空表示不修改' : '私钥有口令时填写，可留空'"/>
          </el-form-item>
        </template>
        <el-form-item label="备注">
          <el-input v-model="connForm.Remark" type="textarea" :rows="2" placeholder="可选"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showConnModal = false">取消</el-button>
        <el-button type="primary" @click="submitConn">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showForwardModal" :title="isEditForward ? '编辑转发规则' : '新建转发规则'" width="620px">
      <el-form :model="forwardForm" label-width="110px">
        <el-form-item label="名称">
          <el-input v-model="forwardForm.Name" placeholder="留空自动生成"/>
        </el-form-item>
        <el-form-item label="转发方向">
          <el-radio-group v-model="forwardForm.Direction" @change="onDirectionChange">
            <el-radio-button value="local">本地转发 -L</el-radio-button>
            <el-radio-button value="remote">远程转发 -R</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="SSH 连接">
          <el-select v-model="forwardForm.SshConnectionId" placeholder="请选择" style="width: 100%">
            <el-option
                v-for="conn in sshConns"
                :key="conn.Id"
                :label="`${conn.Name}（${conn.Username}@${conn.Host}:${conn.Port}）`"
                :value="conn.Id"/>
          </el-select>
        </el-form-item>
        <el-form-item :label="isRemoteForm ? '远端监听端口' : '本地监听端口'">
          <el-input-number v-model="forwardForm.Port" :min="1" :max="65535" controls-position="right"/>
          <div class="text-secondary text-xs mt-05">
            {{ isRemoteForm ? '在 SSH 服务器侧监听的端口' : '在本机监听的端口' }}
          </div>
        </el-form-item>
        <el-form-item label="监听地址">
          <el-input
              v-model="forwardForm.BindAddress"
              :placeholder="isRemoteForm ? '留空默认 127.0.0.1（服务器侧）' : '留空默认 0.0.0.0（本机全网卡）'"/>
          <div v-if="bindAddressRisk" class="text-secondary text-xs mt-05">{{ bindAddressRisk }}</div>
        </el-form-item>
        <el-form-item label="目标主机">
          <el-input
              v-model="forwardForm.TargetHost"
              :placeholder="isRemoteForm ? '由本机侧解析，如 127.0.0.1' : '由 SSH 服务器侧解析，如 127.0.0.1 或 mysql.internal'"/>
          <div class="text-secondary text-xs mt-05">
            {{ isRemoteForm
              ? '远程转发：服务器上接入的流量会回连到本机的该地址'
              : '本地转发：目标地址由 SSH 服务器侧解析' }}
          </div>
        </el-form-item>
        <el-form-item label="目标端口">
          <el-input-number v-model="forwardForm.TargetPort" :min="1" :max="65535" controls-position="right"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="forwardForm.Remark" type="textarea" :rows="2" placeholder="可选"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForwardModal = false">取消</el-button>
        <el-button type="primary" @click="submitForward">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.portfwd-table {
  width: 100%;
}

.portfwd-table :deep(.cell),
.portfwd-table :deep(td.el-table__cell) {
  color: var(--term-green) !important;
}
</style>
