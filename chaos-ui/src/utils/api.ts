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