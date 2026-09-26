<script setup lang="ts">
// SSH 连接管理界面：配置驱动表格 + 内置 CRUD（参考 StandardData 基线）。
// 仅声明 columns 与 fields，凭据字段经 showIf 按认证方式切换；测试连接经 #actions 槽自定义。
import {ElMessage} from 'element-plus'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, FormField} from '@/components/dataTable/types'
import {sshConnApi, type SshConnection} from '@/api/sshConn'

const columns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 140, searchable: true},
  {field: 'Host', title: '主机', minWidth: 180, searchable: true,
    formatter: (row: any) => `${row.Username}@${row.Host}:${row.Port}`},
  {field: 'AuthType', title: '认证方式', width: 100,
    formatter: (row: any) => (row.AuthType === 'key' ? '私钥' : '密码')},
  {field: 'Remark', title: '备注', minWidth: 140, searchable: true},
]

const fields: FormField[] = [
  {field: 'Name', title: '名称', type: 'text', span: 12, required: true, placeholder: '如：跳板机 / 生产服务器'},
  {field: 'Host', title: 'SSH 主机', type: 'text', span: 12, required: true, placeholder: '如 192.168.1.10 或 example.com'},
  {field: 'Port', title: 'SSH 端口', type: 'number', span: 12, min: 1, defaultValue: 22},
  {field: 'Username', title: '用户名', type: 'text', span: 12, required: true, placeholder: '如 root'},
  {field: 'AuthType', title: '认证方式', type: 'select', span: 12, required: true, defaultValue: 'password',
    options: [{label: '密码', value: 'password'}, {label: '私钥', value: 'key'}]},
  {field: 'Password', title: '密码', type: 'password', span: 12, placeholder: '留空表示不修改',
    showIf: {field: 'AuthType', equals: 'password'}},
  {field: 'PrivateKey', title: '私钥', type: 'textarea', rows: 6, placeholder: '留空表示不修改；粘贴 PEM 私钥内容',
    showIf: {field: 'AuthType', equals: 'key'}},
  {field: 'Passphrase', title: '私钥口令', type: 'password', span: 12, placeholder: '留空表示不修改',
    showIf: {field: 'AuthType', equals: 'key'}},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, placeholder: '可选'},
]

// 从 sendMessage 抛出的错误体中提取可读信息
function pickError(e: unknown): string {
  const raw = e instanceof Error ? e.message : String(e)
  const idx = raw.indexOf('{')
  if (idx >= 0) {
    try {
      const parsed = JSON.parse(raw.slice(idx))
      if (parsed && typeof parsed.error === 'string' && parsed.error) return parsed.error
    } catch {
      /* 非 JSON，回退默认提示 */
    }
  }
  return raw
}

async function testConn(row: SshConnection) {
  try {
    const res = await sshConnApi.test(row.ID)
    const version = res?.data?.serverVersion ? ` (${res.data.serverVersion})` : ''
    ElMessage.success(`连接成功${version}`)
  } catch (e) {
    ElMessage.error(pickError(e))
  }
}
</script>

<template>
  <div>
    <DataTable
        :columns="columns"
        :api="sshConnApi"
        :fields="fields"
        title="SSH 连接"
        row-key="ID"
    >
      <template #actions="{row}">
        <el-button size="small" text @click="testConn(row)">测试</el-button>
      </template>
    </DataTable>
  </div>
</template>
