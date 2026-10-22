// filelink 前端插件：路由与菜单 meta；后端 internal/domain/filelink/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const filelinkPlugin: Plugin = {
  meta: { id: 'filelink', name: '文件链接', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/fileLink',
      name: 'fileLink',
      component: () => import('./FileLink.vue'),
      meta: { title: '文件链接', icon: 'Link' },
    },
  ],
}

registerPlugin(filelinkPlugin)
