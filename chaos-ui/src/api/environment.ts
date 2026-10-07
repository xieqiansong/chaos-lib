// 环境变量（envVariables）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位（与标准数据 baseline 一致）。
// 接口重构后统一走 action()（POST + Action，/api/v1/env-variables/*）。
import {action} from '@/utils/request'

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
    return action<EnvResponse>('env-variables', 'get')
}

/** 将当前系统环境变量同步到快照 */
export function syncEnvVariables(): Promise<unknown> {
    return action('env-variables', 'sync')
}

/** 增量修改环境变量：set 新增/覆盖，unset 删除；外层 key 为 scope（'system' | 'user'） */
export interface EnvPatchPayload {
    [scope: string]: {
        set?: Record<string, string>
        unset?: string[]
    }
}

export function patchEnvVariables(payload: EnvPatchPayload): Promise<EnvResponse | null> {
    return action<EnvResponse | null>('env-variables', 'patch', payload)
}

/** 构造一个作用域补丁负载（set 新增/覆盖，unset 删除）；未提供的段不出现。 */
export function buildPatch(scope: 'system' | 'user', set?: Record<string, string>, unset?: string[]): EnvPatchPayload {
    const section: { set?: Record<string, string>; unset?: string[] } = {}
    if (set) section.set = set
    if (unset) section.unset = unset
    return {[scope]: section}
}
