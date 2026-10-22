// datadict 前端插件：路由与菜单 meta；后端 internal/domain/dict/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const datadictPlugin: Plugin = {
    meta: { id: 'datadict', name: '数据字典', version: '1.0.0', dependencies: [] },
    routes: [
        {
            path: '/dataDict',
            name: 'dataDict',
            component: () => import('@/views/DataDict.vue'),
            meta: { title: '数据字典', icon: 'Collection' },
        },
    ],
}

registerPlugin(datadictPlugin)
