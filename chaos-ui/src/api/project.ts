// 项目（project）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
// 标准 CRUD 由通用 useRestApi 提供；移动 / 访问 / 认领（创建）属基线之外的扩展能力，本文件自行追加。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export interface Project {
  ID: number
  GroupID: number
  Name: string
  AbsolutePath: string
  RelativePath: string
  GitURL: string | null
  Remark: string | null
  LastAccessedAt: string | null
  CreatedAt: string
  /** 列表派生：已认领 / 未认领（磁盘未入库目录） */
  Claimed?: boolean
}

const rest = useRestApi<Project>('projects')

export const projectApi = {
  ...rest,
  // 移动：同卷 rename / 跨卷 copy（对应后端 PATCH /projects/:id/move）
  move: (id: number, TargetGroupID: number, TargetRelativePath?: string) =>
    sendMessage(`projects/${id}/move`, 'PATCH', {TargetGroupID, TargetRelativePath}),
  // 访问：记录访问时间（对应后端 PATCH /projects/:id/access）
  access: (id: number) => sendMessage(`projects/${id}/access`, 'PATCH', {}),
  // 认领：把磁盘未入库目录登记为项目（即带绝对路径的创建）
  claim: (item: Pick<Project, 'GroupID' | 'Name' | 'AbsolutePath' | 'RelativePath'>) =>
    rest.create(item),
}
