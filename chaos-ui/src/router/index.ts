import {createRouter, createWebHashHistory, type RouteRecordRaw} from 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** 菜单 / 面包屑标题 */
    title?: string
    /** Element Plus 图标组件名（已全局注册） */
    icon?: string
    /** 为 true 时不生成菜单项（仍可正常访问） */
    hidden?: boolean
    /** 为 true 时脱离常规布局，全屏渲染 */
    fullscreen?: boolean
  }
}

/**
 * 业务路由表：meta.title / meta.icon 是侧边菜单与面包屑的唯一数据来源，
 * 新增页面只需在此追加一条路由，菜单自动出现。
 */
export const appRoutes: RouteRecordRaw[] = [
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: {title: '看板', icon: 'Odometer'},
  },
  {
    path: '/task',
    name: 'task',
    component: () => import('@/views/Task.vue'),
    meta: {title: '任务管理', icon: 'Tickets'},
  },
  {
    path: '/projectManage',
    name: 'projectManage',
    component: () => import('@/views/ProjectManage.vue'),
    meta: {title: '项目管理', icon: 'Folder'},
  },
  {
    path: '/sdk',
    name: 'sdk',
    component: () => import('@/views/Sdk.vue'),
    meta: {title: 'SDK版本', icon: 'Collection'},
  },
  {
    path: '/fileLink',
    name: 'fileLink',
    component: () => import('@/views/FileLink.vue'),
    meta: {title: '文件连接', icon: 'Link'},
  },
  {
    path: '/portForward',
    name: 'portForward',
    component: () => import('@/views/PortForward.vue'),
    meta: {title: '端口转发', icon: 'Connection'},
  },
  {
    path: '/quickEdit',
    name: 'quickEdit',
    component: () => import('@/views/QuickEdit.vue'),
    meta: {title: '快速编辑', icon: 'EditPen'},
  },
  {
    path: '/environment',
    name: 'environment',
    component: () => import('@/views/Environment.vue'),
    meta: {title: '环境变量', icon: 'Setting'},
  },
  {
    path: '/browserHistory',
    name: 'browserHistory',
    component: () => import('@/views/BrowserHistory.vue'),
    meta: {title: '历史记录', icon: 'Clock', hidden: true},
  },
  {
    path: '/example',
    name: 'example',
    component: () => import('@/views/Example.vue'),
    meta: {title: '测试例子', icon: 'MagicStick', hidden: true},
  },
  {
    path: '/board',
    name: 'board',
    component: () => import('@/views/MobileBoard.vue'),
    meta: {title: '全屏看板', icon: 'Monitor', hidden: true, fullscreen: true},
  },
]

const routes: RouteRecordRaw[] = [
  {path: '/', redirect: '/dashboard'},
  ...appRoutes,
  {path: '/:pathMatch(.*)*', redirect: '/dashboard'},
]

export interface MenuNode {
  name: string
  path: string
  title: string
  icon?: string
  children: MenuNode[]
}

function resolvePath(base: string, path: string) {
  if (path.startsWith('/')) return path
  return `${base}/${path}`.replace(/\/{2,}/g, '/')
}

/**
 * 由路由表自动生成菜单树：
 * - 跳过 meta.hidden
 * - 无 meta.title 的容器路由不显示自身，直接摊平其子项
 * - 子路由自动拼接父级 path
 */
export function buildMenu(routes: readonly RouteRecordRaw[] = appRoutes, base = ''): MenuNode[] {
  const nodes: MenuNode[] = []
  for (const route of routes) {
    const path = resolvePath(base, route.path)
    const children = route.children ? buildMenu(route.children, path) : []
    const meta = route.meta || {}
    if (meta.hidden) continue
    if (meta.title) {
      nodes.push({name: String(route.name ?? path), path, title: meta.title, icon: meta.icon, children})
    } else {
      nodes.push(...children)
    }
  }
  return nodes
}

/** 拍平菜单树（供命令面板使用） */
export function flattenMenu(nodes: MenuNode[]): MenuNode[] {
  return nodes.flatMap(node => [node, ...flattenMenu(node.children)])
}

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  scrollBehavior: () => ({top: 0}),
})

export default router
