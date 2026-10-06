// 文件连接（fileLinks）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
// 标准 CRUD 已统一走 action()（见 useRestApi）；状态切换（status）亦迁移为 POST + Action。
import {action} from '@/utils/request'
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

const rest = useRestApi<FileLink>('file-links')

export const fileLinkApi = {
    ...rest,
    // 自定义接口：状态切换（对应后端 POST /api/v1/file-links/status）
    setStatus: (id: number, status: boolean) => action<unknown>('file-links', 'status', {id, status}),
}
