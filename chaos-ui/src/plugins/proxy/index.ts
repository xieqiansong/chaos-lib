// proxy 前端插件：聚合浏览器历史 / 书签的 UI 路由与菜单 meta；接口仍走既有 api.ts（action('proxy', ...)）。
// 后端 internal/domain/proxy/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const proxyPlugin: Plugin = {
  meta: {
    id: 'proxy',
    name: '代理服务',
    version: '1.0.0',
    dependencies: [],
  },
  routes: [
    {
      path: '/browserHistory',
      name: 'browserHistory',
      component: () => import('@/views/BrowserHistory.vue'),
      meta: { title: '历史记录', icon: 'Clock', hidden: true },
    },
    {
      path: '/bookmarks',
      name: 'bookmarks',
      component: () => import('@/views/Bookmarks.vue'),
      meta: { title: '书签管理', icon: 'Star' },
    },
  ],
}

// 自注册
registerPlugin(proxyPlugin)
