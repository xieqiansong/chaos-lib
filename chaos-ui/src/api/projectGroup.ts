// 项目组（projectGroup）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
import {useRestApi} from '@/composables/useRestApi'

export interface ProjectGroup {
  ID: number
  Name: string
  OrderNum: number
  AbsolutePath: string
  Remark: string | null
  CreatedAt: string
  UpdatedAt: string
}

// 标准 CRUD 资源：列表 / 详情 / 创建 / 更新 / 删除（软删）由通用 useRestApi 提供。
// 删组级联软删子项目由后端 AfterDelete 钩子保证，前端照常调用 remove 即可。
export const projectGroupApi = useRestApi<ProjectGroup>('projectGroups')
