export const API_BASE = (import.meta.env.VITE_API_BASE as string) || '/api'

const RETRY_DELAY = 1000
const MAX_RETRIES = 2

function delay(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms))
}

function buildUrl(base: string, path: string, query?: Record<string, any>): string {
    let url = base + '/' + path
    if (query && Object.keys(query).length > 0) {
        const params = new URLSearchParams()
        for (const key of Object.keys(query)) {
            if (query[key] !== undefined && query[key] !== null) {
                params.append(key, String(query[key]))
            }
        }
        const queryString = params.toString()
        if (queryString) {
            url += (url.includes('?') ? '&' : '?') + queryString
        }
    }
    return url
}

async function fetchWithRetry(url: string, options: RequestInit, retries: number = 0): Promise<Response> {
    try {
        const response = await fetch(url, options)
        if (response.ok) {
            return response
        }
        if (retries < MAX_RETRIES && (response.status >= 500 || response.status === 0)) {
            await delay(RETRY_DELAY)
            return fetchWithRetry(url, options, retries + 1)
        }
        return response
    } catch (error) {
        if (retries < MAX_RETRIES) {
            await delay(RETRY_DELAY)
            return fetchWithRetry(url, options, retries + 1)
        }
        throw error
    }
}

export async function sendMessage(path: string, method: string, payload?: any): Promise<any> {
    const options: RequestInit = {
        method,
        cache: 'no-store',
    }
    let url: string
    if (payload && method === 'GET') {
        url = buildUrl(API_BASE, path, payload)
    } else {
        url = API_BASE + '/' + path
        if (payload) {
            options.headers = {'Content-Type': 'application/json'}
            options.body = JSON.stringify(payload)
        }
    }
    const response = await fetchWithRetry(url, options)
    const contentType = response.headers.get('content-type') || ''
    if (!response.ok) {
        const text = await response.text().catch(() => '')
        throw new Error(`Request failed: ${text}`)
    }
    if (!contentType.includes('application/json')) {
        const text = await response.text().catch(() => '')
        throw new Error(`Request returned non-JSON response. URL: ${url}. Is the backend running? Response preview: ${text.substring(0, 200)}`)
    }
    return response.json()
}

// ---- SDK 版本切换 ----

export interface SdkInfo {
    CurrentVersion: string
    VersionList: string[]
}

export function getSdkVersions(): Promise<Record<string, SdkInfo>> {
    return sendMessage('sdks', 'GET')
}

export function updateSdkVersion(type: string, version: string): Promise<any> {
    return sendMessage(`sdks/${type}/switch`, 'PATCH', { version })
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
    return sendMessage('sdks/defs', 'GET')
}

export function createSdkDef(payload: {
    Name: string
    Sources: SdkSourceItem[]
    Enabled?: boolean
    Note?: string
}): Promise<SdkSource> {
    return sendMessage('sdks/defs', 'POST', payload)
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
    return sendMessage(`sdks/defs/${name}`, 'PATCH', payload)
}

export function deleteSdkDef(name: string): Promise<any> {
    return sendMessage(`sdks/defs/${name}`, 'DELETE')
}

// ---- SSH 连接与端口转发 ----

export type SshAuthType = 'password' | 'key'

export interface SshConnection {
    Id: number
    Name: string
    Host: string
    Port: number
    Username: string
    AuthType: SshAuthType
    Remark: string
    /** 是否已配置凭据（后端不回传明文） */
    HasPassword: boolean
    HasPrivateKey: boolean
    HasPassphrase: boolean
}

export interface SshConnectionPayload {
    Name: string
    Host: string
    Port: number
    Username: string
    AuthType: SshAuthType
    /** 为空表示保持原值（编辑时） */
    Password?: string
    PrivateKey?: string
    Passphrase?: string
    Remark?: string
}

export type PortForwardDirection = 'local' | 'remote' | 'direct'

export interface PortForward {
    Id: number
    Name: string
    /** local = 本机监听（ssh -L）；remote = SSH 服务器侧监听（ssh -R）；direct = 本机监听直连目标（纯 TCP） */
    Direction: PortForwardDirection
    /** 监听端口：local/direct 为本机端口，remote 为服务器侧端口 */
    Port: number
    /** 监听地址：local/direct 缺省 0.0.0.0，remote 缺省 127.0.0.1 */
    BindAddress: string
    TargetHost: string
    TargetPort: number
    /** direct 方向无需 SSH 连接，此字段为 0 */
    SshConnectionId: number
    /** 以内存实际运行状态为准 */
    Status: boolean
    LastError: string
    Remark: string
}

export interface PortForwardPayload {
    Name?: string
    Direction?: PortForwardDirection
    Port: number
    BindAddress?: string
    TargetHost: string
    TargetPort: number
    /** direct 方向传 0 或不传 */
    SshConnectionId: number
    Remark?: string
}

export function getSshConns(): Promise<SshConnection[]> {
    return sendMessage('sshConns', 'GET')
}

export function createSshConn(payload: SshConnectionPayload): Promise<any> {
    return sendMessage('sshConns', 'POST', payload)
}

export function updateSshConn(id: number, payload: Partial<SshConnectionPayload>): Promise<any> {
    return sendMessage(`sshConns/${id}`, 'PATCH', payload)
}

export function deleteSshConn(id: number): Promise<any> {
    return sendMessage(`sshConns/${id}`, 'DELETE')
}

export function testSshConn(id: number): Promise<any> {
    return sendMessage(`sshConns/${id}/test`, 'POST', {})
}

export function getPortForwards(): Promise<PortForward[]> {
    return sendMessage('portForwards', 'GET')
}

export function createPortForward(payload: PortForwardPayload): Promise<any> {
    return sendMessage('portForwards', 'POST', payload)
}

export function updatePortForward(id: number, payload: Partial<PortForwardPayload>): Promise<any> {
    return sendMessage(`portForwards/${id}`, 'PATCH', payload)
}

export function deletePortForward(id: number): Promise<any> {
    return sendMessage(`portForwards/${id}`, 'DELETE')
}

export function updatePortForwardStatus(id: number, status: boolean): Promise<any> {
    return sendMessage(`portForwards/${id}/status`, 'PATCH', {status})
}

// ---- MQTT 多节点同步 ----

export interface MqttMessage {
    msg_id: string
    node_id: string
    channel: string
    payload: string
    created_at: string
    /** 是否为本机发出的消息 */
    is_self: boolean
}

export interface MqttStatus {
    enabled: boolean
    connected: boolean
    broker: string
    prefix: string
    node_id: string
    encrypt: boolean
}

export function getMqttMessages(): Promise<MqttMessage[]> {
    return sendMessage('mqttSync/messages', 'GET')
}

export function sendMqttMessage(payload: string, channel?: string): Promise<MqttMessage> {
    return sendMessage('mqttSync/messages', 'POST', { payload, channel })
}

export function deleteMqttMessagesByChannel(channel: string): Promise<{ channel: string; deleted: number }> {
    return sendMessage(`mqttSync/messages?channel=${encodeURIComponent(channel)}`, 'DELETE')
}

export function getMqttStatus(): Promise<MqttStatus> {
    return sendMessage('mqttSync/status', 'GET')
}

// ---- 数据库监控 ----

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
    return sendMessage('dbMonitor/overview', 'GET')
}

export function getTables(): Promise<{ items: TableStat[]; total: number }> {
    return sendMessage('dbMonitor/tables', 'GET')
}

export function getTableDetail(name: string): Promise<TableDetail> {
    return sendMessage(`dbMonitor/tables/${encodeURIComponent(name)}`, 'GET')
}

// ---- 待办任务 ----

export function postponeTask(id: number, days: number): Promise<any> {
    return sendMessage(`tasks/${id}/postpone`, 'PATCH', { days })
}

export function batchPostponeTasks(ids: number[], days: number): Promise<any> {
    return sendMessage('tasks/batch-postpone', 'POST', { ids, days })
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

/** 拉取浏览器历史；size 控制返回条数（后端按 last_visit_time DESC 排序）。
 *  后端遵循统一分页规范，返回 { items, total, page, size }，此处仅取出 items。
 *  无 size 时取 MaxSize=200，近似「全量」，供常用书签基于全量历史按访问频率排序。 */
export function getBrowserHistories(size?: number): Promise<BrowserHistoryItem[]> {
  const query: Record<string, any> = { size: size && size > 0 ? size : 200 }
  return sendMessage('browserHistories', 'GET', query).then((res: any) => res?.items ?? res)
}

/** 按关键词全文搜索浏览器历史（标题 / URL）。取较大分页近似「全部命中」，避免前端搜索态截断。 */
export function searchBrowserHistories(q: string): Promise<BrowserHistoryItem[]> {
  return sendMessage('browserHistories', 'GET', {search: q, size: 200}).then((res: any) => res?.items ?? res)
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
  items: T[]
  total: number
  page: number
  size: number
}

/** 拉取「常用书签」：按访问频率降序分页（默认前 20 条）。search 可选，按标题/URL 模糊匹配。 */
export function getFrequentBookmarks(page = 1, size = 20, search?: string): Promise<PagedResult<FrequentBookmarkItem>> {
  const query: Record<string, any> = {page, size}
  if (search && search instanceof String) query.search = search
  return sendMessage('frequentBookmarks', 'GET', query)
}

// ---- 主机名（用作浏览器标签标题） ----

export interface HostnameInfo {
    hostname: string
    username: string
}

export function getHostname(): Promise<HostnameInfo> {
    return sendMessage('hostname', 'GET')
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
