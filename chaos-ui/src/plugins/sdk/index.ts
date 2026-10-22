// sdk 前端插件：路由与菜单 meta；后端 internal/domain/sdk/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const sdkPlugin: Plugin = {
  meta: { id: 'sdk', name: 'SDK版本', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/sdk',
      name: 'sdk',
      component: () => import('./Sdk.vue'),
      meta: { title: 'SDK版本', icon: 'Collection' },
    },
  ],
}

registerPlugin(sdkPlugin)
