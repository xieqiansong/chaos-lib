// 个人物品模块的接口门户：集中承载类型与 api 函数。
import {action} from '@/utils/request'
import {useRestApi, toSnake} from '@/composables/useRestApi'
import type {DataTableApiParams} from '@/components/dataTable/types'

// ── 类型 ────────────────────────────────────────────────
export interface Category {
    ID: number
    ParentID: number
    Name: string
    Icon: string | null
    SortOrder: number
    CreatedAt: string
    UpdatedAt: string
    IsDeleted: boolean
}

export interface Location {
    ID: number
    ParentID: number
    Name: string
    SortOrder: number
    CreatedAt: string
    UpdatedAt: string
    IsDeleted: boolean
}

export interface Item {
    ID: number
    Name: string
    CategoryID: number | null
    LocationID: number | null
    ItemType: number
    Brand: string | null
    Model: string | null
    Spec: string | null
    Color: string | null
    Quantity: number
    Unit: string
    MinQuantity: number | null
    Status: string
    SerialNo: string | null
    PurchaseDate: string | null
    PurchasePrice: number | null
    PurchaseChannel: string | null
    WarrantyEnd: string | null
    ExpireDate: string | null
    LastReplenishDate: string | null
    LastReplenishQty: number | null
    Tags: string | null
    CoverImage: string | null
    Remark: string | null
    CreatedAt: string
    UpdatedAt: string
    IsDeleted: boolean
}

export interface ItemRecord {
    ID: number
    ItemID: number
    RecordType: string
    Quantity: number | null
    Amount: number | null
    RecordDate: string
    Note: string | null
    CreatedAt: string
    IsDeleted: boolean
}

export interface ItemAssembly {
    ID: number
    HostID: number
    ComponentID: number
    Slot: string | null
    Quantity: number
    InstalledAt: string
    RemovedAt: string | null
    InstallNote: string | null
    RemoveNote: string | null
    CreatedAt: string
    IsDeleted: boolean
}

export interface ComponentView {
    ComponentID: number
    Name: string
    Brand: string | null
    Model: string | null
    Spec: string | null
    SerialNo: string | null
    Slot: string | null
    Quantity: number
    InstalledAt: string
}

export interface AssemblyHistory {
    HostID: number
    HostName: string
    Slot: string | null
    InstalledAt: string
    RemovedAt: string | null
    State: string
}

export interface CategoryStat {
    CategoryID: number
    CategoryName: string | null
    Count: number
    TotalValue: number
}

export interface ReplenishCycle {
    ItemID: number
    ItemName: string
    Cycles: number
    AvgCycleDays: number
    LastBuy: string
    NextBuyEstimate: string | null
}

// ── 通用 CRUD（分类 / 位置 / 物品 / 流水 / 装配）────────────
export const categoriesApi = useRestApi<Category>('categories')
export const locationsApi = useRestApi<Location>('locations')
export const itemsApi = useRestApi<Item>('items')
export const itemRecordsApi = useRestApi<ItemRecord>('item-records')
export const itemAssembliesApi = useRestApi<ItemAssembly>('item-assemblies')

// 列表查询封装（与 DataTable 兼容）
async function listFetch<T>(prefix: string, params: DataTableApiParams) {
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
    const res = await action<{ list: T[]; pagination: { total: number } }>(prefix, 'list', {}, meta)
    const rows = (res?.list ?? []).map((it: any) => ({...it, ID: it.ID ?? it.Id})) as T[]
    return {rows, total: res?.pagination?.total ?? rows.length}
}

export const itemRecordsFetch = (params: DataTableApiParams) => listFetch<ItemRecord>('item-records', params)
export const itemAssembliesFetch = (params: DataTableApiParams) => listFetch<ItemAssembly>('item-assemblies', params)

// ── 业务动作（事务型）────────────────────────────────────
export const itemActions = {
    purchase: (data: any) => action<Item>('items', 'purchase', data),
    replenish: (data: any) => action<Item>('items', 'replenish', data),
    consume: (data: any) => action<Item>('items', 'consume', data),
    install: (data: any) => action<null>('items', 'install', data),
    remove: (data: any) => action<null>('items', 'remove', data),
    lend: (data: any) => action<null>('items', 'lend', data),
    return: (data: any) => action<null>('items', 'return', data),
    repair: (data: any) => action<null>('items', 'repair', data),
    dispose: (data: any) => action<null>('items', 'dispose', data),
    inventory: (data: any) => action<Item>('items', 'inventory', data),
}

// ── 派生查询 ────────────────────────────────────────────
export const itemQueries = {
    lowStock: () => action<Item[]>('items', 'lowStock', {}),
    warrantySoon: (days = 30) => action<Item[]>('items', 'warrantySoon', {days}),
    categoryStats: () => action<CategoryStat[]>('items', 'categoryStats', {}),
    hostConfig: (hostId: number) => action<ComponentView[]>('items', 'hostConfig', {hostId}),
    componentHistory: (componentId: number) => action<AssemblyHistory[]>('items', 'componentHistory', {componentId}),
    replenishForecast: () => action<ReplenishCycle[]>('items', 'replenishForecast', {}),
}

// 选项常量（前端展示用）
export const ITEM_TYPE_OPTIONS = [
    {label: '耐用品', value: 1},
    {label: '消耗品', value: 2},
]
export const STATUS_OPTIONS = [
    {label: '在用', value: 'in_use'},
    {label: '闲置', value: 'idle'},
    {label: '借出', value: 'lent'},
    {label: '维修', value: 'repairing'},
    {label: '损坏', value: 'broken'},
    {label: '已处置', value: 'disposed'},
]
export const RECORD_TYPE_LABEL: Record<string, string> = {
    purchase: '购入', consume: '消耗', replenish: '补货', lend: '借出', return: '归还',
    repair: '维修', move: '转移', dispose: '处置', inventory: '盘点', install: '装入', remove: '拆下',
}
export const STATUS_LABEL: Record<string, string> = {
    in_use: '在用', idle: '闲置', lent: '借出', repairing: '维修', broken: '损坏', disposed: '已处置',
}
export const SOURCE_OPTIONS = [
    {label: '淘宝', value: '淘宝'},
    {label: '京东', value: '京东'},
    {label: '闲鱼', value: '闲鱼'},
    {label: '拼多多', value: '拼多多'},
    {label: '线下', value: '线下'},
    {label: '其他', value: '其他'},
]
