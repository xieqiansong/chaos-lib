// items 前端插件：路由与菜单 meta；后端 internal/domain/items/plugin.go 已对应登记。
import {registerPlugin} from '../registry'
import type {Plugin} from '../types'

export const itemsPlugin: Plugin = {
    meta: {id: 'items', name: '个人物品', version: '1.0.0', dependencies: []},
    routes: [
        {
            path: '/items',
            name: 'items',
            component: () => import('./Items.vue'),
            meta: {title: '个人物品', icon: 'Box'},
        },

    ],
}

registerPlugin(itemsPlugin)
