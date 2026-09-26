// SSH 连接接口门户：本文件集中承载该资源的「接口契约」。视图只从这里导入类型与 api 对象。
// 后端接入 crud 基线（internal/portfwd + internal/crud），列表/详情/创建/更新/删除走标准 CRUD，
// 测试连接为业务自定义接口（POST /sshConns/:id/test）。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export type SshAuthType = 'password' | 'key'

export interface SshConnection {
  ID: number
  Name: string
  Host: string
  Port: number
  Username: string
  AuthType: SshAuthType
  Remark: string
  /** 凭据仅后端使用，响应一律脱敏，仅回传是否已配置标记 */
  HasPassword: boolean
  HasPrivateKey: boolean
  HasPassphrase: boolean
}

export interface SshConnectionPayload {
  Name: string
  Host: string
  Port: number
  Username: string
  AuthType: SshAuthType
  /** 为空（编辑时）表示保持原值 */
  Password?: string
  PrivateKey?: string
  Passphrase?: string
  Remark?: string
}

const rest = useRestApi<SshConnection>('sshConns')

export const sshConnApi = {
  ...rest,
  // 编辑时留空的凭据表示保持原值：剔除空字段，避免清空后端现有凭据
  update: (id: number, data: Partial<SshConnectionPayload>) => {
    const patch: Record<string, any> = {}
    for (const [k, v] of Object.entries(data)) {
      if (v === '' || v === undefined || v === null) continue
      patch[k] = v
    }
    return rest.update(id, patch)
  },
  // 测试连接：不启动端口转发，仅验证主机可达性与凭据有效性
  test: (id: number) => sendMessage(`sshConns/${id}/test`, 'POST', {}),
}
