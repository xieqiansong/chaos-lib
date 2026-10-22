// polyform 前端插件：路由与菜单 meta；后端 internal/domain/polyform/plugin.go 已对应登记（占位）。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const polyformPlugin: Plugin = {
  meta: { id: 'polyform', name: '格式转换', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/polyform',
      name: 'polyform',
      component: () => import('@/views/Polyform.vue'),
      meta: { title: '格式转换', icon: 'Sort' },
    },
  ],
}

registerPlugin(polyformPlugin)
