// datacache 前端插件：路由与菜单 meta；后端 internal/framework/datacache/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const datacachePlugin: Plugin = {
  meta: { id: 'datacache', name: '数据缓存', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/dataCache',
      name: 'dataCache',
      component: () => import('./DataCache.vue'),
      meta: { title: '数据缓存', icon: 'Collection' },
    },
  ],
}

registerPlugin(datacachePlugin)
