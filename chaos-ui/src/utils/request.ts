import axios, {type AxiosInstance, type AxiosRequestConfig, type AxiosResponse, type InternalAxiosRequestConfig,} from 'axios'
import router from '@/router'
import {showError} from '@/utils/message'

/** 后端接口返回的统一结构（按仓库《接口规范.md》：code === 0 成功，requestId/timestamp 由框架回填） */
export interface ApiResponse<T = unknown> {
    code: number
    message: string
    data: T
    /** 本次请求的唯一 ID，链路追踪与幂等用（存量接口也会由后端补全） */
    requestId?: string
    /** 响应毫秒时间戳 */
    timestamp?: number
}

/** 统一请求信封（POST + Action 模式），详见仓库《接口规范.md》 */
export interface ActionEnvelope {
    requestId: string
    /** 动作名，形如 `user.create` */
    action: string
    /** 业务数据 */
    data: unknown
    /** 分页/排序/来源等附加信息 */
    meta?: unknown
    /** 请求毫秒时间戳 */
    timestamp: number
}

/** 生成请求唯一 ID（用于幂等、链路追踪）：优先原生 crypto.randomUUID，降级到随机数 */
function genRequestId(): string {
    try {
        if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
            return crypto.randomUUID()
        }
    } catch {
        // 忽略，走降级
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = (Math.random() * 16) | 0
        const v = c === 'x' ? r : (r & 0x3) | 0x8
        return v.toString(16)
    })
}

const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

const request: AxiosInstance = axios.create({
    baseURL,
    timeout: 15000,
    headers: {
        'Content-Type': 'application/json',
    },
})

// 请求拦截器：统一附加鉴权 token
request.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
        const token = localStorage.getItem('token')
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    },
    (error) => Promise.reject(error),
)

// 响应拦截器：剥离统一结构，统一错误处理
request.interceptors.response.use(
    (response: AxiosResponse<ApiResponse>) => {
        const res = response.data
        // 约定 code === 0 为成功；按实际后端调整
        if (res && typeof res.code === 'number' && res.code !== 0) {
            showError(res.message || '请求失败')
            return Promise.reject(new Error(res.message || '请求失败'))
        }
        return response
    },
    (error) => {
        if (error.response?.status === 401) {
            localStorage.removeItem('token')
            showError('登录已过期，请重新登录')
            // 未授权：清空 token 后跳转登录页（SPA 导航，避免整页刷新）
            if (router.currentRoute.value.path !== '/login') {
                router.push('/login')
            }
        } else if (error.response) {
            // 服务端有响应但非 2xx
            showError(`请求失败（${error.response.status}）`)
        } else if (error.code === 'ECONNABORTED') {
            showError('请求超时，请稍后重试')
        } else {
            showError('网络错误，请检查连接')
        }
        return Promise.reject(error)
    },
)

/** 泛型封装：直接返回 data 业务字段（已剥离 AxiosResponse 与统一结构外层） */
export function get<T = any>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    let fullUrl = buildUrl(url, data);
    return request.get<ApiResponse<T>>(fullUrl, config).then((r) => r.data.data)
}

export function post<T = any>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return request.post<ApiResponse<T>>(url, data, config).then((r) => r.data.data)
}

export function put<T = any>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return request.put<ApiResponse<T>>(url, data, config).then((r) => r.data.data)
}

export function del<T = any>(url: string, config?: AxiosRequestConfig) {
    return request.delete<ApiResponse<T>>(url, config).then((r) => r.data.data)
}

export function patch<T = any>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return request.patch<ApiResponse<T>>(url, data, config).then((r) => r.data.data)
}

/**
 * action 调用：新接口「POST + Action」统一入口（详见《接口规范.md》）。
 * 向 POST /api/v1/{module}/{action} 发送带信封的请求体，自动注入 requestId 与 timestamp。
 * 语义与 get/post 等保持一致——成功时只返回 data 业务字段（信封外层已在拦截器处理）。
 *
 * 示例：await action('user', 'create', { username: 'zhangsan' })
 * 对应后端：POST /api/v1/user/create，body 为 { requestId, action: 'user.create', data: {...}, timestamp }
 */
export function action<T = any>(
    module: string,
    act: string,
    data?: unknown,
    meta?: unknown,
    config?: AxiosRequestConfig,
): Promise<T> {
    const envelope: ActionEnvelope = {
        requestId: genRequestId(),
        action: `${module}.${act}`,
        data: data ?? {},
        meta,
        timestamp: Date.now(),
    }
    return request
        .post<ApiResponse<T>>(`/v1/${module}/${act}`, envelope, config)
        .then((r) => r.data.data)
}

function buildUrl(path: string, query?: Record<string, any>): string {
    let url = path
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

export default request
