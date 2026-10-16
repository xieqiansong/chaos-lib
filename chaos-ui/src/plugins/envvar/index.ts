// envvar 前端插件：路由与菜单 meta；后端 internal/domain/envvar/plugin.go 已对应登记。
// 注：envvar 以 TOML/虚拟文件存储，无 GORM 模型，后端 Models 留空，但前端照常贡献路由。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const envvarPlugin: Plugin = {
  meta: { id: 'envvar', name: '环境变量', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/environment',
      name: 'environment',
      component: () => import('@/views/Environment.vue'),
      meta: { title: '环境变量', icon: 'Setting' },
    },
  ],
}

registerPlugin(envvarPlugin)
