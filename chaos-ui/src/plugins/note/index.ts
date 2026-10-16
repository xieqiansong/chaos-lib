// note 前端插件：拥有自己的路由与菜单 meta；接口仍走既有 api/note.ts。
// 后端 internal/domain/note/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const notePlugin: Plugin = {
  meta: {
    id: 'note',
    name: '笔记',
    version: '1.0.0',
    dependencies: [],
  },
  routes: [
    {
      path: '/notes',
      name: 'notes',
      component: () => import('@/views/Notes.vue'),
      meta: { title: '笔记', icon: 'Notebook' },
    },
  ],
}

// 自注册
registerPlugin(notePlugin)
