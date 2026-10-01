// 仪表盘（Dashboard）的接口门户：本文件集中承载该视图依赖的接口契约。
// 视图只从这里导入类型与 api 函数，搜索该业务时一键定位（与标准数据 baseline 一致）。
import {get} from '@/utils/request'

// ---- DeepSeek 余额（服务端代理，直接透传 DeepSeek 原始响应）----

export interface DeepSeekBalanceResponse {
    balance_infos?: { total_balance?: number | string }[]
    [key: string]: any
}

/**
 * 查询 DeepSeek 账户余额。该接口为服务端透传：成功时直接返回 DeepSeek 原始 JSON（无统一信封），
 * 失败时返回标准错误信封（{ code, message }）。故此处用 fetch 读取原始响应体，并把错误抛给视图
 * 以内联文本展示，而非触发全局错误 toast。
 */
export async function getDeepSeekBalance(): Promise<DeepSeekBalanceResponse> {
    const res = await fetch('/api/balance/deepseek')
    const data = await res.json()
    if (!res.ok) {
        throw new Error(data?.message || data?.error || '未知错误')
    }
    return data
}

// ---- 任务贡献统计（GitHub 风格热力图）----

export interface ContributionItem {
    id: number
    name: string
    total: number
    days: { date: string; count: number }[]
}

export interface ContributionStats {
    rootName: string
    start: string
    end: string
    items: ContributionItem[]
}

export function getContributionStats(): Promise<ContributionStats> {
    return get<ContributionStats>('tasks/contributionStats')
}

// ---- 每日完成任务数（折线图） / 每日待办任务数（直方图）----
// 两个接口均返回同一形状的点数组：{ date, count }

export interface TaskStatPoint {
    date: string
    count: number
}

/** 近 N 天每日完成任务数（默认 29 天，由后端回退） */
export function getDailyStats(days: number): Promise<TaskStatPoint[]> {
    return get<TaskStatPoint[]>('tasks/dailyStats', {days})
}

/** 指定日期区间内每日待办（进行中）任务数 */
export function getActiveStats(start: string, end: string): Promise<TaskStatPoint[]> {
    return get<TaskStatPoint[]>('tasks/activeStats', {start, end})
}
