<script setup lang="ts">
// 端口转发规则管理界面：配置驱动表格 + 内置 CRUD（参考 StandardData 基线）。
// 两张表拆为两个界面，本页只管「转发规则」；SSH 连接另见 SshConn.vue。
// 状态启停走自定义 setStatus 接口（标准 CRUD 之外），经 switch-handler 注入 DataTable。
import {computed, onMounted, ref} from 'vue'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {portForwardApi, type PortForward, type PortForwardDirection} from '@/api/portForward'
import {sshConnApi, type SshConnection} from '@/api/sshConn'

// 关联 SSH 连接列表：用于表单下拉与列表展示
const sshConns = ref<SshConnection[]>([])
const connNameById = computed(() => {
  const m: Record<number, string> = {}
  for (const c of sshConns.value) m[c.ID] = c.Name
  return m
})
const connOptions = computed(() =>
  sshConns.value.map(c => ({
    label: `${c.Name}（${c.Username}@${c.Host}:${c.Port}）`,
    value: c.ID,
  })),
)

onMounted(async () => {
  try {
    const res = await sshConnApi.fetch({page: 1, pageSize: 1000, search: {}, sort: undefined})
    sshConns.value = res.rows
  } catch {
    /* 连接列表加载失败不影响转发规则页本身 */
  }
})

const directionMeta: Record<PortForwardDirection, {label: string; tag: string}> = {
  local: {label: '本地 -L', tag: 'info'},
  remote: {label: '远程 -R', tag: 'warning'},
  direct: {label: '直接转发', tag: 'success'},
}

const columns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 150, searchable: true},
  {field: 'Direction', title: '方向', width: 110, searchable: true,
    search: {type: 'select', options: [
      {label: '本地 -L', value: 'local'},
      {label: '远程 -R', value: 'remote'},
      {label: '直接转发', value: 'direct'},
    ]},
    formatter: (row: any) => directionMeta[row.Direction as PortForwardDirection]?.label || row.Direction},
  {field: 'Port', title: '监听端口', width: 130,
    formatter: (row: any) => `${row.Port}（${row.Direction === 'remote' ? '服务器侧' : '本机'}）`},
  {field: 'BindAddress', title: '监听地址', width: 130},
  {field: 'TargetHost', title: '目标', minWidth: 160, searchable: true,
    formatter: (row: any) => `${row.TargetHost}:${row.TargetPort}`},
  {field: 'SshConnectionId', title: 'SSH 连接', width: 150,
    formatter: (row: any) =>
      row.Direction === 'direct' ? '—' : (connNameById.value[row.SshConnectionId] || `#${row.SshConnectionId}`)},
  {field: 'Status', title: '状态', width: 90, type: 'switch'},
  {field: 'LastError', title: '最近错误', minWidth: 160,
    formatter: (row: any) => row.LastError || '-'},
  {field: 'Remark', title: '备注', minWidth: 120, searchable: true},
]

// 表单字段：SshConnectionId 仅非 direct 时显示，选项来自关联连接列表（随加载结果更新）
const fields = computed<FormField[]>(() => [
  {field: 'Name', title: '名称', type: 'text', span: 12, placeholder: '留空自动生成'},
  {field: 'Direction', title: '转发方向', type: 'select', span: 12, required: true, defaultValue: 'local',
    options: [
      {label: '本地转发 -L', value: 'local'},
      {label: '远程转发 -R', value: 'remote'},
      {label: '直接转发', value: 'direct'},
    ]},
  {field: 'Port', title: '监听端口', type: 'number', span: 12, min: 1, required: true},
  {field: 'BindAddress', title: '监听地址', type: 'text', span: 12, placeholder: '留空按方向取默认'},
  {field: 'SshConnectionId', title: 'SSH 连接', type: 'select', span: 12, defaultValue: 0,
    options: connOptions.value, placeholder: '请选择',
    showIf: {field: 'Direction', notEquals: 'direct'}},
  {field: 'TargetHost', title: '目标主机', type: 'text', span: 12, required: true, placeholder: '由本机/服务器侧解析，如 127.0.0.1'},
  {field: 'TargetPort', title: '目标端口', type: 'number', span: 12, min: 1, required: true},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, placeholder: '可选'},
])

// 状态启停：回调返回 Promise，成功后 DataTable 自动刷新并提示
function onStatusChange(row: PortForward, next: boolean) {
  return portForwardApi.setStatus(row.ID, next)
}
</script>

<template>
  <div>
    <DataTable
        :columns="columns"
        :api="portForwardApi"
        :fields="fields"
        title="端口转发"
        row-key="ID"
        :switch-handler="onStatusChange"
    />
  </div>
</template>
