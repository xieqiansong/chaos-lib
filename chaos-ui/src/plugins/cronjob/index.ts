// cronjob 前端插件：路由与菜单 meta；后端 internal/domain/cronjob/plugin.go 已对应登记，
// 并在 Start 钩子里启动 cron 调度运行时。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const cronjobPlugin: Plugin = {
  meta: { id: 'cronjob', name: '定时任务', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/cronJob',
      name: 'cronJob',
      component: () => import('@/views/CronJob.vue'),
      meta: { title: '定时任务', icon: 'AlarmClock' },
    },
  ],
}

registerPlugin(cronjobPlugin)
