// MQTT 多节点同步（mqttSync 资源）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
// 标准 CRUD（list/get/create/update/delete）由 useRestApi 生成，驱动 DataTable；
// 集群状态、按主题删除为扩展接口，由本资源自实现（对应后端 /mqttSync/status、/mqttSync/channel）。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export interface MqttSyncMessage {
  ID: number
  MsgID: string
  NodeID: string
  Channel: string
  Payload: string
  /** 是否为本机发出的消息（由后端的 NodeID 比对得出） */
  IsSelf: boolean
  CreatedAt: string
  UpdatedAt: string
}

export interface MqttStatus {
  enabled: boolean
  connected: boolean
  broker: string
  prefix: string
  node_id: string
  encrypt: boolean
}

const rest = useRestApi<MqttSyncMessage>('mqttSync')

export const mqttSyncApi = {
  ...rest,
  // 集群状态：GET /mqttSync/status
  status: () => sendMessage('mqttSync/status', 'GET') as Promise<MqttStatus>,
  // 按主题删除（软删）：DELETE /mqttSync/channel?name=
  deleteChannel: (name: string) =>
    sendMessage(`mqttSync/channel?name=${encodeURIComponent(name)}`, 'DELETE'),
}
