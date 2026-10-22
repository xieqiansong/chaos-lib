// dashboard 前端插件：路由与菜单 meta；后端 internal/domain/dashboard/plugin.go 已对应登记（占位）。
// dashboard 为前端聚合视图（依赖 proxy / task-plans 等既有模块接口），无独立后端模型与路由。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const dashboardPlugin: Plugin = {
  meta: { id: 'dashboard', name: '看板', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('./Dashboard.vue'),
      meta: { title: '看板', icon: 'Odometer' },
    },
  ],
}

registerPlugin(dashboardPlugin)
