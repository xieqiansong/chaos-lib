import {createRouter, createWebHistory, type RouteRecordRaw} from 'vue-router'

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
    path: '/',
    name: 'home',
    component: () => import('@/views/Home.vue'),
    meta: {title: '首页', icon: 'HomeFilled'},
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: {title: '看板', icon: 'Odometer'},
  },
  {
    path: '/taskPlan',
    name: 'taskPlan',
    component: () => import('@/views/TaskPlan.vue'),
    meta: {title: '任务计划', icon: 'Tickets'},
  },
  {
    path: '/pendingTask',
    name: 'pendingTask',
    component: () => import('@/views/PendingTask.vue'),
    meta: {title: '待办任务', icon: 'Clock'},
  },
  {
    // 兼容旧地址：原「任务管理」页已拆为任务计划 / 待办任务
    path: '/task',
    redirect: '/taskPlan',
    meta: {hidden: true},
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
    // 临时测试界面：用通用 DataTable 组件复刻文件连接，验证配置驱动表格方案
    path: '/fileLinkTest',
    name: 'fileLinkTest',
    component: () => import('@/views/FileLinkTest.vue'),
    meta: {title: '文件连接-临时测试', icon: 'Link'},
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
  {
    path: '/mqttSync',
    name: 'mqttSync',
    component: () => import('@/views/MqttSync.vue'),
    meta: {title: 'MQTT同步', icon: 'Connection'},
  },
  {
    path: '/polyform',
    name: 'polyform',
    component: () => import('@/views/Polyform.vue'),
    meta: {title: '格式转换', icon: 'Sort'},
  },
  {
    path: '/databaseMonitor',
    name: 'databaseMonitor',
    component: () => import('@/views/DatabaseMonitor.vue'),
    meta: {title: '数据监控', icon: 'Coin'},
  },
  {
    path: '/bookmarks',
    name: 'bookmarks',
    component: () => import('@/views/Bookmarks.vue'),
    meta: {title: '书签管理', icon: 'Star'},
  },
  {
    path: '/cronJob',
    name: 'cronJob',
    component: () => import('@/views/CronJob.vue'),
    meta: {title: '定时任务', icon: 'AlarmClock'},
  },
]

const routes: RouteRecordRaw[] = [
  ...appRoutes,
  {path: '/:pathMatch(.*)*', redirect: '/'},
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
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({top: 0}),
})

export default router
