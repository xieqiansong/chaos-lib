// 标准参考表（standardData）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
import {action, patch} from '@/utils/request'
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

const rest = useRestApi<StandardData>('standard-data')

export const standardDataApi = {
    ...rest,
    // 自定义接口：状态切换（标准 CRUD 之外，由本资源自实现，对应后端 PATCH /standardData/:id/status）
    setStatus: (id: number, status: boolean) => patch(`standard-data/${id}/status`, {status}),
}

// 标准参考表的「POST + Action」接口（v1，详见《接口规范.md》）。
// 与 standardDataApi 并存（双轨迁移）；字段语义与存量接口一致，仅传输改为统一信封。
export const standardDataApiV1 = {
    list: (meta?: unknown) => action<{ list: StandardData[]; pagination: unknown }>('standard-data', 'list', {}, meta),
    get: (id: number) => action<StandardData>('standard-data', 'get', {id}),
    create: (data: Partial<StandardData>) => action<StandardData>('standard-data', 'create', data),
    update: (id: number, data: Partial<StandardData>) => action<StandardData>('standard-data', 'update', {id, ...data}),
    delete: (id: number) => action<null>('standard-data', 'delete', {id}),
    batchCreate: (items: Partial<StandardData>[]) => action<StandardData[]>('standard-data', 'batchCreate', {items}),
    batchDelete: (ids: number[]) => action<null>('standard-data', 'batchDelete', {ids}),
    setStatus: (id: number, status: boolean) => action<StandardData>('standard-data', 'status', {id, status}),
}
