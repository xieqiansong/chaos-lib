// 文件连接（fileLinks）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export interface FileLink {
  ID: number
  SourcePath: string
  TargetPath: string
  Status: boolean
  Remark: string
  Sort: number
  // 派生字段：由后端在返回时按文件系统实时计算，不落库
  LinkStatus: string
  CreatedAt: string
  UpdatedAt: string
}

const rest = useRestApi<FileLink>('fileLinks')

export const fileLinkApi = {
  ...rest,
  // 自定义接口：状态切换（标准 CRUD 之外，由本资源自实现，对应后端 PATCH /fileLinks/:id/status）
  setStatus: (id: number, status: boolean) => sendMessage(`fileLinks/${id}/status`, 'PATCH', {status}),
}
