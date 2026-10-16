// 前端插件契约：与后端 internal/framework/plugin 同形。
// 传输层（action / useRestApi）完全不动，插件只负责声明、依赖与路由/菜单聚合。
import type { RouteRecordRaw } from 'vue-router'
import type { App } from 'vue'

// 插件元数据：依赖校验与拓扑排序的依据。
export interface PluginMeta {
  id: string
  name: string
  version: string
  dependencies: string[]
  /** 为 true 时禁用；缺省（未设置）即启用，符合「默认全启用」契约；开关能力后续接入 */
  disabled?: boolean
}

// 一个功能模块的前端插件：拥有自己的路由（视图懒加载）、菜单 meta，并可注册全局能力。
export interface Plugin {
  meta: PluginMeta
  /** 本插件贡献的路由，由注册表汇总后交给 vue-router */
  routes: RouteRecordRaw[]
  /** 可选：安装全局能力（如按需注册组件、指令、ECharts 模块等） */
  install?: (app: App) => void
}
