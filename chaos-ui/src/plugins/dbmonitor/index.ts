// dbmonitor 前端插件：路由与菜单 meta；后端 internal/framework/dbmonitor/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const dbmonitorPlugin: Plugin = {
  meta: { id: 'dbmonitor', name: '数据监控', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/databaseMonitor',
      name: 'databaseMonitor',
      component: () => import('@/views/DatabaseMonitor.vue'),
      meta: { title: '数据监控', icon: 'Coin' },
    },
  ],
}

registerPlugin(dbmonitorPlugin)
