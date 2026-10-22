// 数据字典（dataDict）的接口门户：集中承载类型与 api 函数。
// 后端对应 internal/domain/dict（POST + Action 风格，/api/v1/dict-types、/api/v1/dict-data）。
import {action} from '@/utils/request'
import {toSnake} from '@/composables/useRestApi'
import type {DataTableApiParams} from '@/components/dataTable/types'

export interface DictType {
    ID: number
    Name: string
    Code: string
    Status: boolean
    Sort: number
    Remark: string
    CreatedAt: string
    UpdatedAt: string
    IsDeleted: boolean
}

export interface DictData {
    ID: number
    TypeCode: string
    Label: string
    Value: string
    ParentID: number
    Sort: number
    Status: boolean
    Remark: string
    Ext: string
    CreatedAt: string
    UpdatedAt: string
    IsDeleted: boolean
}

export interface DictOption {
    ID: number
    Label: string
    Value: string
    ParentID: number
}

export interface DictNode {
    ID: number
    Label: string
    Value: string
    ParentID: number
    Sort: number
    Status: boolean
    Children?: DictNode[]
}

// 把 DataTable 的查询参数翻译为信封 meta（分页/排序/模糊搜索）。
function buildMeta(params: DataTableApiParams, extra?: Record<string, any>) {
    const meta: Record<string, any> = {page: params.page, pageSize: params.pageSize}
    if (params.sort?.field) {
        meta.sort = toSnake(params.sort.field)
        meta.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
    }
    const like: Record<string, any> = {}
    for (const [k, v] of Object.entries(params.search || {})) {
        if (v !== '' && v !== null && v !== undefined) like[toSnake(k)] = v
    }
    if (Object.keys(like).length) meta.like = like
    if (extra) Object.assign(meta, extra)
    return meta
}

// 解包分页响应：统一补全 ID 字段并抽出 total。
function unpack<T>(res: any): { rows: T[]; total: number } {
    const raw = (res?.list ?? []) as any[]
    const rows = raw.map((it: any) => ({...it, ID: it.ID ?? it.Id})) as T[]
    return {rows, total: res?.pagination?.total ?? rows.length}
}

// 字典类型：完整 CRUD（含状态切换）。
export const dictTypeApi = {
    fetch: async (params: DataTableApiParams) => {
        const res = await action<{ list: DictType[]; pagination: { total: number } }>(
            'dict-types', 'list', {}, buildMeta(params),
        )
        return unpack<DictType>(res)
    },
    create: (data: Partial<DictType>) => action<DictType>('dict-types', 'create', data),
    update: (id: number, data: Partial<DictType>) => action<DictType>('dict-types', 'update', {id, ...data}),
    remove: (id: number) => action<null>('dict-types', 'delete', {id}),
    // 自定义接口：状态切换，对应后端 POST /api/v1/dict-types/status
    setStatus: (id: number, status: boolean) => action<DictType>('dict-types', 'status', {id, status}),
}

// 取某类型全量数据项（不分页），用于父级下拉与父级名称解析。
export async function listDictDataAll(typeCode: string): Promise<DictData[]> {
    const res = await action<{ list: DictData[] }>(
        'dict-data', 'list', {}, {page: 1, pageSize: 9999, filter: {type_code: typeCode}},
    )
    return (res?.list ?? []).map((it: any) => ({...it, ID: it.ID ?? it.Id})) as DictData[]
}

// 取某类型下的启用数据项（下拉源），对应后端 POST /api/v1/dict-data/options。
export async function listDictOptions(typeCode: string): Promise<DictOption[]> {
    const res = await action<DictOption[]>('dict-data', 'options', {typeCode})
    return (res ?? []) as DictOption[]
}

// 取某类型下的数据项树（按 ParentID 嵌套），对应后端 POST /api/v1/dict-data/tree。
export async function listDictTree(typeCode: string): Promise<DictNode[]> {
    const res = await action<DictNode[]>('dict-data', 'tree', {typeCode})
    return (res ?? []) as DictNode[]
}

// 按当前选中的字典类型构造数据项 api（自动注入 TypeCode）。
export function createDictDataApi(typeCode: string) {
    return {
        fetch: async (params: DataTableApiParams) => {
            const res = await action<{ list: DictData[]; pagination: { total: number } }>(
                'dict-data', 'list', {}, buildMeta(params, {filter: {type_code: typeCode}}),
            )
            return unpack<DictData>(res)
        },
        create: (data: Partial<DictData>) => action<DictData>('dict-data', 'create', {typeCode, ...data}),
        update: (id: number, data: Partial<DictData>) => action<DictData>('dict-data', 'update', {id, ...data}),
        remove: (id: number) => action<null>('dict-data', 'delete', {id}),
        // 自定义接口：状态切换，对应后端 POST /api/v1/dict-data/status
        setStatus: (id: number, status: boolean) => action<DictData>('dict-data', 'status', {id, status}),
    }
}
