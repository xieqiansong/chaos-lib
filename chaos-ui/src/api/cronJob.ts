// 定时任务（cronJob）的接口门户：本文件集中承载该资源的「接口契约」。
// 视图只从这里导入类型与 api 对象，搜索该业务时一键定位（与标准数据 baseline 一致）。
import {sendMessage} from '@/utils/api'
import {useRestApi} from '@/composables/useRestApi'

export interface CronJob {
  ID: number
  Name: string
  CronExpr: string
  ActionType: string
  ActionConfig: string
  Enabled: boolean
  TimeoutSec: number
  LastRunAt: string | null
  LastStatus: string
  // 派生字段：由后端按 cron 表达式实时计算，不落库
  NextRun: string | null
  CreatedAt: string
  UpdatedAt: string
}

export interface CronJobRun {
  ID: number
  JobID: number
  StartedAt: string
  FinishedAt: string | null
  Success: boolean
  Output: string
  Error: string
}

const rest = useRestApi<CronJob>('cronJob')

export const cronJobApi = {
  ...rest,
  // 自定义接口：启停（标准 CRUD 之外，由本资源自实现，对应后端 PATCH /cronJob/:id/status）
  setStatus: (id: number, status: boolean) => sendMessage(`cronJob/${id}/status`, 'PATCH', {status}),
  // 立即执行一次，返回本次运行记录
  run: (id: number): Promise<CronJobRun> => sendMessage(`cronJob/${id}/run`, 'POST', {}),
  // 运行历史（分页）
  runs: (id: number, params?: Record<string, any>): Promise<{ items: CronJobRun[]; total: number }> =>
      sendMessage(`cronJob/${id}/runs`, 'GET', params),
  // cron 表达式校验 + 未来若干次触发时间预览
  preview: (cronExpr: string, count = 5): Promise<{ valid: boolean; error?: string; nextRuns: string[] }> =>
      sendMessage('cronJob/preview', 'POST', {cronExpr, count}),
}
