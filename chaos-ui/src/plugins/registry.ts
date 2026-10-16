// 前端插件注册表：登记、依赖校验（缺失依赖 / 循环依赖）、拓扑排序、路由/图标聚合。
// 与后端 registry.go 行为保持一致（Kahn 拓扑排序）。
import type { App, Plugin, PluginMeta, RouteRecordRaw } from './types'

const plugins: Plugin[] = []

// registerPlugin 登记一个插件（应在应用启动前、router 模块求值前完成）。
export function registerPlugin(p: Plugin): void {
  if (!p.meta?.id) {
    throw new Error('plugin: 空 id 的插件登记')
  }
  if (plugins.some((x) => x.meta.id === p.meta.id)) {
    throw new Error(`plugin: 重复登记插件 ${p.meta.id}`)
  }
  plugins.push(p)
}

function isEnabled(m: PluginMeta): boolean {
  return !m.disabled
}

// buildPlugins 做依赖校验 + 拓扑排序，返回依赖有序的启用插件；异常即抛错。
export function buildPlugins(): Plugin[] {
  const byId = new Map<string, Plugin>()
  plugins.forEach((p) => byId.set(p.meta.id, p))

  for (const p of plugins) {
    if (!isEnabled(p.meta)) continue
    for (const dep of p.meta.dependencies) {
      if (!byId.has(dep)) {
        throw new Error(`插件 ${p.meta.id} 依赖未注册的插件 ${dep}`)
      }
    }
  }

  const indeg = new Map<string, number>()
  const adj = new Map<string, string[]>()
  for (const p of plugins) {
    if (!isEnabled(p.meta)) continue
    if (!indeg.has(p.meta.id)) indeg.set(p.meta.id, 0)
    for (const dep of p.meta.dependencies) {
      adj.set(dep, [...(adj.get(dep) ?? []), p.meta.id])
      indeg.set(p.meta.id, (indeg.get(p.meta.id) ?? 0) + 1)
    }
  }

  const queue: string[] = []
  indeg.forEach((d, id) => {
    if (d === 0) queue.push(id)
  })
  const ordered: string[] = []
  while (queue.length) {
    const id = queue.shift()!
    ordered.push(id)
    for (const nxt of adj.get(id) ?? []) {
      const left = (indeg.get(nxt) ?? 0) - 1
      indeg.set(nxt, left)
      if (left === 0) queue.push(nxt)
    }
  }
  if (ordered.length !== indeg.size) {
    const inCycle = [...indeg.entries()].filter(([, d]) => d > 0).map(([id]) => id)
    throw new Error(`检测到插件循环依赖，涉及: ${inCycle.join(', ')}`)
  }

  return ordered.map((id) => byId.get(id)!)
}

// collectRoutes 汇总所有插件路由（catch-all 由 router 末尾追加）。
// 能力型插件（如 echarts）无 routes，用空数组兜底，避免产生 undefined 路由项导致
// vue-router 在创建时抛出 "Cannot read properties of undefined (reading 'path')"。
export function collectRoutes(): RouteRecordRaw[] {
  return buildPlugins().flatMap((p) => p.routes ?? [])
}

// collectIcons 收集所有被插件路由 meta.icon 引用的图标名，供 main.ts 自动注册。
// 新增插件只要在路由 meta.icon 写 Element Plus 图标名，即自动可用，无需维护白名单。
export function collectIcons(): string[] {
  const icons = new Set<string>()
  for (const p of buildPlugins()) {
    for (const r of p.routes ?? []) {
      const icon = (r.meta as Record<string, unknown> | undefined)?.icon
      if (typeof icon === 'string') icons.add(icon)
    }
  }
  return [...icons]
}

// installPlugins 依次执行各插件 install 钩子（注册全局能力）。
export function installPlugins(app: App): void {
  for (const p of buildPlugins()) {
    p.install?.(app)
  }
}
