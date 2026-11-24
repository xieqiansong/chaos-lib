import {action} from '@/utils/request'

export interface SdkInfo {
    CurrentVersion: string
    VersionList: string[]
}

export function getSdkVersions(): Promise<Record<string, SdkInfo>> {
    return action('sdks', 'list')
}

export function updateSdkVersion(type: string, version: string): Promise<any> {
    return action('sdks', 'switch', {type, version})
}

export interface SdkSourceItem {
    kind: 'repo' | 'single'
    root: string
}

export interface SdkSource {
    ID: number
    Name: string
    Sources: SdkSourceItem[]
    Current: string
    Enabled: boolean
    Note: string
    IsDeleted: boolean
}

export function getSdkDefs(): Promise<SdkSource[]> {
    return action('sdks', 'listSources')
}

export function createSdkDef(payload: {
    Name: string
    Sources: SdkSourceItem[]
    Enabled?: boolean
    Note?: string
}): Promise<SdkSource> {
    return action('sdks', 'createSource', payload)
}

export function updateSdkDef(
    name: string,
    payload: {
        Sources?: SdkSourceItem[]
        Current?: string
        Enabled?: boolean
        Note?: string
    }
): Promise<SdkSource> {
    return action('sdks', 'updateSource', {name, SourcePatch: payload})
}

export function deleteSdkDef(name: string): Promise<any> {
    return action('sdks', 'deleteSource', {name})
}

export function batchPostponeTasks(ids: number[], days: number): Promise<any> {
    return action('task-plans', 'tasks/batchPostpone', {ids, days})
}

export interface BrowserHistoryItem {
    ID: string
    LastVisitTime: number
    Title: string
    TypeCount: number
    Url: string
    VisitCount: number
}

/** 拉取浏览器历史；page_size 控制返回条数（后端按 last_visit_time DESC 排序）。
 *  后端遵循统一分页规范，返回 { code, message, data:{ list, pagination } }，此处仅取出 list。
 *  无 page_size 时取 MaxPageSize=200，近似「全量」，供常用书签基于全量历史按访问频率排序。 */
export function getBrowserHistories(page_size?: number): Promise<BrowserHistoryItem[]> {
    const meta: Record<string, any> = {page: 1, page_size: page_size && page_size > 0 ? page_size : 200}
    return action<any>('proxy', 'browserHistories/list', {}, meta).then((res: any) => res?.list ?? [])
}

/** 按关键词全文搜索浏览器历史（标题 / URL）。取较大分页近似「全部命中」，避免前端搜索态截断。 */
export function searchBrowserHistories(q: string): Promise<BrowserHistoryItem[]> {
    return action<any>('proxy', 'browserHistories/list', {}, {page: 1, page_size: 200, search: q}).then((res: any) => res?.list ?? [])
}

export interface FrequentBookmarkItem {
    id: string
    title: string
    url: string
    /** 关联的历史访问次数 */
    count: number
    /** 关联的历史最近访问时间（Chrome 微秒时间戳） */
    last: number
}

export interface PagedResult<T> {
    list: T[]
    pagination: {
        page: number
        page_size: number
        total: number
        total_pages: number
    }
}

/** 拉取「常用书签」：按访问频率降序分页（默认前 20 条）。search 可选，按标题/URL 模糊匹配。
 *  入参 page + page_size；响应为统一信封的 data，即 { list, pagination }。 */
export function getFrequentBookmarks(page = 1, page_size = 20, search?: any): Promise<PagedResult<FrequentBookmarkItem>> {
    const meta: Record<string, any> = {page, page_size}
    if (search && typeof search === "string") {
        meta.search = search
    }
    return action<any>('proxy', 'bookmarks/frequent', {}, meta)
}

export interface BookmarkTreeNode {
    id: string
    parentId?: string
    title: string
    url?: string
    isFolder: boolean
    sortIndex: number
    dateAdded: number
    children?: BookmarkTreeNode[]
}

/** 从后端数据库拉取完整书签树（嵌套结构）。 */
export function getBookmarkTree(): Promise<BookmarkTreeNode[]> {
    return action<BookmarkTreeNode[]>('proxy', 'bookmarks/getTree')
}

export interface HostnameInfo {
    hostname: string
    username: string
}

export function getHostname(): Promise<HostnameInfo> {
    return action<HostnameInfo>('proxy', 'hostname')
}

