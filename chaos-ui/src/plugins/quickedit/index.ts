// quickedit 前端插件：路由与菜单 meta；后端 internal/domain/quickedit/plugin.go 已对应登记。
// 软依赖 envvar（运行期经回调注入虚拟文件服务），前端依赖图不强制，故 dependencies 留空。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const quickeditPlugin: Plugin = {
  meta: { id: 'quickedit', name: '快速编辑', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/quickEdit',
      name: 'quickEdit',
      component: () => import('./QuickEdit.vue'),
      meta: { title: '快速编辑', icon: 'EditPen' },
    },
  ],
}

registerPlugin(quickeditPlugin)
