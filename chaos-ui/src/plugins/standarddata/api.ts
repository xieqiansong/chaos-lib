// 标准参考表（standardData）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位。
import {action} from '@/utils/request'
import {toSnake} from '@/composables/useRestApi'
import type {DataTableApiParams} from '@/components/dataTable/types'

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

// standardDataApi 已切换为「POST + Action」实现（v1，详见《接口规范.md》）：
// 方法名保持与通用 DataTable 兼容（fetch/create/update/remove/setStatus），内部全部走统一信封，
// 后端落在 /api/v1/standard-data。存量 /api/standard-data RESTful 路由仍保留，可随时回退。
export const standardDataApi = {
    fetch: async (params: DataTableApiParams) => {
        const meta: Record<string, any> = {
            page: params.page,
            pageSize: params.pageSize,
        }
        if (params.sort?.field) {
            meta.sort = toSnake(params.sort.field)
            meta.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
        }
        const search = params.search || {}
        const filter: Record<string, any> = {}
        for (const [k, v] of Object.entries(search)) {
            if (v !== '' && v !== null && v !== undefined) {
                filter[toSnake(k)] = v
            }
        }
        if (Object.keys(filter).length > 0) {
            meta.filter = filter
        }
        const res = await action<{ list: StandardData[]; pagination: { total: number } }>(
            'standard-data', 'list', {}, meta,
        )
        const rows = (res?.list ?? []).map((it: any) => ({...it, ID: it.ID ?? it.Id})) as StandardData[]
        return {rows, total: res?.pagination?.total ?? rows.length}
    },
    create: (data: Partial<StandardData>) => action<StandardData>('standard-data', 'create', data),
    update: (id: number, data: Partial<StandardData>) => action<StandardData>('standard-data', 'update', {id, ...data}),
    remove: (id: number) => action<null>('standard-data', 'delete', {id}),
    // 自定义接口：状态切换，对应后端 POST /api/v1/standard-data/status
    setStatus: (id: number, status: boolean) => action<StandardData>('standard-data', 'status', {id, status}),
}
