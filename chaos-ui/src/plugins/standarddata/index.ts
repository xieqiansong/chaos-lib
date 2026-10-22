// standarddata 前端插件：路由与菜单 meta；后端 internal/domain/standarddata/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const standarddataPlugin: Plugin = {
  meta: { id: 'standarddata', name: '标准数据', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/standardData',
      name: 'standardData',
      component: () => import('./StandardData.vue'),
      meta: { title: '标准数据', icon: 'Grid' },
    },
  ],
}

registerPlugin(standarddataPlugin)
