import {createRouter, createWebHistory, type RouteRecordRaw} from 'vue-router'
import '@/plugins' // 触发所有插件登记（必须在 collectRoutes 之前求值）
import {collectRoutes} from '@/plugins/registry'

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
// 应用级路由（非业务插件）：落地页、隐藏的测试页与全屏页。
// 业务功能路由已全部迁入各自前端插件（见 src/plugins/*），由 collectRoutes 汇总。
export const appRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/views/Home.vue'),
    meta: {title: '首页', icon: 'HomeFilled'},
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

// 聚合「插件路由 + 应用级路由」；业务路由已全部迁入各自插件，appRoutes 仅剩应用级页面。
const allAppRoutes: RouteRecordRaw[] = [...appRoutes, ...collectRoutes()]

const routes: RouteRecordRaw[] = [
  ...allAppRoutes,
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
export function buildMenu(routes: readonly RouteRecordRaw[] = allAppRoutes, base = ''): MenuNode[] {
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
