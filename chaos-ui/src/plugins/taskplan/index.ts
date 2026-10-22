// taskplan 前端插件：拥有自己的路由与菜单 meta；接口仍走既有 api/taskPlan.ts。
// 作为插件化改造的参照实现：后端 internal/domain/taskplan/plugin.go 已对应登记。
// 每个插件自注册（与后端各包 init() 调 plugin.Register 对齐），聚合入口只做副作用导入。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const taskplanPlugin: Plugin = {
  meta: {
    id: 'taskplan',
    name: '任务计划',
    version: '1.0.0',
    dependencies: [],
  },
  routes: [
    {
      path: '/taskPlan',
      name: 'taskPlan',
      component: () => import('./TaskPlan.vue'),
      meta: { title: '任务计划', icon: 'Tickets' },
    },
    {
      path: '/pendingTask',
      name: 'pendingTask',
      component: () => import('./PendingTask.vue'),
      meta: { title: '待办任务', icon: 'Clock' },
    },
    {
      // 兼容旧地址：原「任务管理」页已拆为任务计划 / 待办任务
      path: '/task',
      redirect: '/taskPlan',
      meta: { hidden: true },
    },
  ],
}

// 自注册
registerPlugin(taskplanPlugin)
