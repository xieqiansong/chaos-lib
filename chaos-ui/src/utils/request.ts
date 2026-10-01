import axios, {
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import router from '@/router'
import { showError } from '@/utils/message'

/** 后端接口返回的统一结构（按 chaos-nestjs 约定：code === 0 成功） */
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
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
export function get<T = unknown>(url: string, config?: AxiosRequestConfig) {
  return request.get<ApiResponse<T>>(url, config).then((r) => r.data.data)
}

export function post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
  return request.post<ApiResponse<T>>(url, data, config).then((r) => r.data.data)
}

export function put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) {
  return request.put<ApiResponse<T>>(url, data, config).then((r) => r.data.data)
}

export function del<T = unknown>(url: string, config?: AxiosRequestConfig) {
  return request.delete<ApiResponse<T>>(url, config).then((r) => r.data.data)
}

export default request
