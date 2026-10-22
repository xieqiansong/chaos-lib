// 仪表盘（Dashboard）的接口门户：本文件集中承载该视图依赖的接口契约。
// 视图只从这里导入类型与 api 函数，搜索该业务时一键定位（与标准数据 baseline 一致）。
// 接口重构后统一走 action()（POST + Action）：
//   - DeepSeek 余额：/api/v1/proxy/balance/deepseek（后端已包信封，成功返回原始余额对象）
//   - 任务统计：/api/v1/task-plans/tasks/{contributionStats|dailyStats|activeStats}
import {action} from '@/utils/request'

export interface DeepSeekBalanceResponse {
    balance_infos?: { total_balance?: number | string }[]
    [key: string]: any
}

/**
 * 查询 DeepSeek 账户余额。后端代理并把原始 JSON 包进统一信封，故此处直接走 action()；
 * 失败时后端返回非 0 code，由 axios 拦截器抛错，视图以内联文本展示，不触发全局错误 toast。
 */
export async function getDeepSeekBalance(): Promise<DeepSeekBalanceResponse> {
    return action<DeepSeekBalanceResponse>('proxy', 'balance/deepseek')
}

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
    return action<ContributionStats>('task-plans', 'tasks/contributionStats')
}

// 两个接口均返回同一形状的点数组：{ date, count }
export interface TaskStatPoint {
    date: string
    count: number
}

/** 近 N 天每日完成任务数（默认 29 天，由后端回退） */
export function getDailyStats(days: number): Promise<TaskStatPoint[]> {
    return action<TaskStatPoint[]>('task-plans', 'tasks/dailyStats', {}, {days})
}

/** 指定日期区间内每日待办（进行中）任务数 */
export function getActiveStats(start: string, end: string): Promise<TaskStatPoint[]> {
    return action<TaskStatPoint[]>('task-plans', 'tasks/activeStats', {}, {start, end})
}
