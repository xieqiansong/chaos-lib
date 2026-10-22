// project 前端插件：拥有自己的路由与菜单 meta；接口仍走既有 api/project.ts。
// 后端 internal/domain/project/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const projectPlugin: Plugin = {
  meta: {
    id: 'project',
    name: '项目管理',
    version: '1.0.0',
    dependencies: [],
  },
  routes: [
    {
      path: '/projectManage',
      name: 'projectManage',
      component: () => import('./ProjectManage.vue'),
      meta: { title: '项目管理', icon: 'Folder' },
    },
  ],
}

// 自注册
registerPlugin(projectPlugin)
