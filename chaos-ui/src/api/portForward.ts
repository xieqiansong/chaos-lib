// 端口转发规则接口门户：本文件集中承载该资源的「接口契约」。视图只从这里导入类型与 api 对象。
// 后端接入 crud 基线（internal/portfwd + internal/crud），列表/详情/创建/更新/删除走标准 CRUD，
// 启停为业务自定义接口（PATCH /portForwards/:id/status）。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export type PortForwardDirection = 'local' | 'remote' | 'direct'

export interface PortForward {
  ID: number
  Name: string
  /** local = 本机监听（ssh -L）；remote = SSH 服务器侧监听（ssh -R）；direct = 本机监听直连目标（纯 TCP） */
  Direction: PortForwardDirection
  /** 监听端口：local/direct 为本机端口，remote 为服务器侧端口 */
  Port: number
  /** 监听地址：local/direct 缺省 0.0.0.0，remote 缺省 127.0.0.1 */
  BindAddress: string
  TargetHost: string
  TargetPort: number
  /** direct 方向无需 SSH 连接，此字段为 0 */
  SshConnectionId: number
  /** 以内存实际运行状态为准 */
  Status: boolean
  LastError: string
  Remark: string
}

const rest = useRestApi<PortForward>('portForwards')

export const portForwardApi = {
  ...rest,
  // 启停单条转发：local/remote 经关联 SSH 连接建立隧道，direct 直接在本机监听并直连目标
  setStatus: (id: number, status: boolean) => sendMessage(`portForwards/${id}/status`, 'PATCH', {status}),
}
