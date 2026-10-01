// 环境变量（envVariables）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位（与标准数据 baseline 一致）。
import {get, patch, post} from '@/utils/request'

export interface EnvResponse {
    Meta: { SavedAt: string; Hostname: string; Username: string }
    System: Record<string, string>
    User: Record<string, string>
    SnapshotId: number
    SnapshotTime: string
    Warnings: string[]
}

/** 读取全部环境变量（用户 + 系统），以及快照元信息 */
export function getEnvVariables(): Promise<EnvResponse> {
    return get<EnvResponse>('envVariables')
}

/** 将当前系统环境变量同步到快照 */
export function syncEnvVariables(): Promise<unknown> {
    return post('envVariables/sync', {})
}

/** 增量修改环境变量：set 新增/覆盖，unset 删除；外层 key 为 scope（'system' | 'user'） */
export interface EnvPatchPayload {
    [scope: string]: {
        set?: Record<string, string>
        unset?: string[]
    }
}

export function patchEnvVariables(payload: EnvPatchPayload): Promise<EnvResponse | null> {
    return patch<EnvResponse | null>('envVariables', payload)
}
