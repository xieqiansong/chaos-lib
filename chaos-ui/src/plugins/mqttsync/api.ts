// MQTT 多节点同步（mqttSync 资源）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
// 标准 CRUD 已统一走 action()（见 useRestApi）；集群状态 / 按主题删除亦迁移为 POST + Action。
import {action} from '@/utils/request'
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

const rest = useRestApi<MqttSyncMessage>('mqtt-syncs')

export const mqttSyncApi = {
    ...rest,
    // 集群状态：对应后端 POST /api/v1/mqtt-syncs/status
    status: () => action<MqttStatus>('mqtt-syncs', 'status'),
    // 按主题删除（软删）：对应后端 POST /api/v1/mqtt-syncs/deleteChannel
    deleteChannel: (name: string) => action<unknown>('mqtt-syncs', 'deleteChannel', {name}),
}
