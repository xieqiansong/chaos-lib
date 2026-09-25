// 标准参考表（standardData）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
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

export const standardDataApi = useRestApi<StandardData>('standardData')
