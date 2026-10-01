// 数据库监控（dbMonitor）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 函数，搜索本业务接口一键定位（与标准参考表 baseline 一致）。
import {get} from '@/utils/request'

export interface DbOverview {
    dbType: string
    version: string
    totalBytes: number
    tableCount: number
    totalRows: number
    sizeSupported: boolean
}

export interface TableStat {
    name: string
    rows: number
    rowsEstimated: boolean
    tableBytes: number
    indexBytes: number
    totalBytes: number
    indexCount: number
    sizeSupported: boolean
    seqScan?: number | null
    idxScan?: number | null
    lastVacuum?: string | null
    lastAnalyze?: string | null
}

export interface ColumnInfo {
    name: string
    type: string
    nullable: boolean
    isPk: boolean
    default?: string | null
}

export interface IndexInfo {
    name: string
    unique: boolean
    columns: string
}

export interface TableDetail extends TableStat {
    columns: ColumnInfo[]
    indexes: IndexInfo[]
}

export function getDbOverview(): Promise<DbOverview> {
    return get('dbMonitor/overview')
}

// 分页列表（真实分页接口）：透传 page/page_size/sort/order/name，响应为统一信封的 data { list, pagination }。
export function getTables(params?: Record<string, any>): Promise<{
    list: TableStat[];
    pagination: { page: number; page_size: number; total: number; total_pages: number }
}> {
    return get('dbMonitor/tables', params)
}

export function getTableDetail(name: string): Promise<TableDetail> {
    return get(`dbMonitor/tables/${encodeURIComponent(name)}`)
}
