export const API_BASE = (import.meta.env.VITE_API_BASE as string) || '/api'

import {del, get, patch, post} from '@/utils/request'


// ---- SDK 版本切换 ----

export interface SdkInfo {
    CurrentVersion: string
    VersionList: string[]
}

export function getSdkVersions(): Promise<Record<string, SdkInfo>> {
    return get('sdks')
}

export function updateSdkVersion(type: string, version: string): Promise<any> {
    return patch(`sdks/${type}/switch`, {version})
}

// ---- SDK 类型 / 来源管理 (defs) ----

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
    return get('sdks/defs')
}

export function createSdkDef(payload: {
    Name: string
    Sources: SdkSourceItem[]
    Enabled?: boolean
    Note?: string
}): Promise<SdkSource> {
    return post('sdks/defs', payload)
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
    return patch(`sdks/defs/${name}`, payload)
}

export function deleteSdkDef(name: string): Promise<any> {
    return del(`sdks/defs/${name}`)
}

// ---- 待办任务 ----

export function batchPostponeTasks(ids: number[], days: number): Promise<any> {
    return post('tasks/batch-postpone', {ids, days})
}

// ---- 浏览器历史（由扩展周期性备份到 DB）----

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
    const query: Record<string, any> = {page_size: page_size && page_size > 0 ? page_size : 200}
    return get('browserHistories', query).then((res: any) => res?.list ?? [])
}

/** 按关键词全文搜索浏览器历史（标题 / URL）。取较大分页近似「全部命中」，避免前端搜索态截断。 */
export function searchBrowserHistories(q: string): Promise<BrowserHistoryItem[]> {
    return get('browserHistories', {search: q, page_size: 200}).then((res: any) => res?.list ?? [])
}

// ---- 常用书签（独立接口：书签 ∪ 历史访问次数，按访问频率降序，分页）----

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
    const query: Record<string, any> = {page, page_size}
    if (search && typeof search === "string") {
        query.search = search
    }
    return get('frequentBookmarks', query)
}

// ---- 主机名（用作浏览器标签标题） ----

export interface HostnameInfo {
    hostname: string
    username: string
}

export function getHostname(): Promise<HostnameInfo> {
    return get('hostname')
}

// ---- 浏览器扩展直连通道（externally_connectable）----

/** 扩展连接状态：mode 固定为 external（网页直连），connected 反映直连是否可用 */
export interface ExtStatus {
    connected: number
    mode?: 'external'
}

/** 下发给扩展的指令（经 chrome.runtime.sendMessage 直发，可携带任意参数） */
export interface ExtCommand {
    type: string
    url?: string
    text?: string
    id?: string

    [key: string]: any
}

/** 下发指令的返回：pushed 恒为 1（直连即已送达），response 为扩展执行结果 */
export interface ExtPushResult {
    pushed: number
    response?: ExtResponse | null
}

/** 扩展回传的一条记录 */
export interface ExtResponse {
    id?: string
    type: string
    ok: boolean
    error?: string
    echo?: any
    receivedAt?: string
}

/** 读取已配置的扩展 ID（externally_connectable 直连用）。空串表示未配置。 */
export function getExtensionId(): string {
    try {
        const ls = localStorage.getItem('chaos_ext_id')
        if (ls) return ls
    } catch {
        /* ignore */
    }
    return (import.meta.env.VITE_EXTENSION_ID as string) || ''
}

/** 经 externally_connectable 向扩展发一条 ping，探测直连通道是否可用。 */
export async function pingExtension(): Promise<boolean> {
    const id = getExtensionId()
    const chromeRt = (window as any).chrome?.runtime
    if (!id || !chromeRt?.sendMessage) return false
    try {
        const resp = await chromeRt.sendMessage(id, {type: 'ping', id: 'ping-ui'})
        return !!(resp && resp.ok)
    } catch {
        return false
    }
}

/** 获取扩展直连状态（用 ping 探测）。 */
export async function refreshExtStatus(): Promise<ExtStatus> {
    const ok = await pingExtension()
    return {connected: ok ? 1 : 0, mode: 'external'}
}

/**
 * 下发指令到扩展（externally_connectable 直连）。
 * 浏览器收到消息会自动唤醒 MV3 service worker 并执行，结果经 sendResponse 回包。
 * 返回的 response 即扩展执行结果；pushed 恒为 1（直连即已送达）。
 */
export async function pushExtCommand(cmd: ExtCommand): Promise<ExtPushResult> {
    const id = getExtensionId()
    const chromeRt = (window as any).chrome?.runtime
    if (!id || !chromeRt?.sendMessage) {
        throw new Error('未配置扩展直连 ID，无法下发指令（请在书签管理页填写扩展 ID）')
    }
    const resp = await chromeRt.sendMessage(id, cmd)
    return {pushed: 1, response: resp ?? null}
}
