// 标准参考表（standardData）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export interface StandardData {
  ID: number
  Name: string
  Code: string
  Description: string
  Category: string
  Quantity: number
  Price: number
  Enabled: boolean
  Config: string
  EffectiveAt: string | null
  Sort: number
  CreatedAt: string
  UpdatedAt: string
  IsDeleted: boolean
}

const rest = useRestApi<StandardData>('standardData')

export const standardDataApi = {
  ...rest,
  // 自定义接口：状态切换（标准 CRUD 之外，由本资源自实现，对应后端 PATCH /standardData/:id/status）
  setStatus: (id: number, status: boolean) => sendMessage(`standardData/${id}/status`, 'PATCH', {status}),
}
