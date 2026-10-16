// portfwd 前端插件：拥有自己的路由与菜单 meta；接口仍走既有 api/portForward.ts。
// 后端 internal/domain/portfwd/plugin.go 已对应登记，并在 Start 钩子里恢复隧道。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const portfwdPlugin: Plugin = {
  meta: {
    id: 'portfwd',
    name: '端口转发',
    version: '1.0.0',
    dependencies: [],
  },
  routes: [
    {
      path: '/portForward',
      name: 'portForward',
      component: () => import('@/views/PortForward.vue'),
      meta: { title: '端口转发', icon: 'Connection' },
    },
  ],
}

// 自注册
registerPlugin(portfwdPlugin)
