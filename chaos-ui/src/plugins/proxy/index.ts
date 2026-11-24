// proxy 前端插件：浏览器历史的 UI 路由与菜单 meta；接口仍走既有 api.ts（action('proxy', ...)）。
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
      component: () => import('./BrowserHistory.vue'),
      meta: { title: '历史记录', icon: 'Clock', hidden: true },
    },
  ],
}

// 自注册
registerPlugin(proxyPlugin)
