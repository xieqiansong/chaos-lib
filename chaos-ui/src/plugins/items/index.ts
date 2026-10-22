// items 前端插件：路由与菜单 meta；后端 internal/domain/items/plugin.go 已对应登记。
import {registerPlugin} from '../registry'
import type {Plugin} from '../types'

export const itemsPlugin: Plugin = {
    meta: {id: 'items', name: '个人物品', version: '1.0.0', dependencies: []},
    routes: [
        {
            path: '/items',
            name: 'items',
            component: () => import('@/views/Items.vue'),
            meta: {title: '个人物品', icon: 'Box'},
        },
        {
            path: '/itemCategories',
            name: 'itemCategories',
            component: () => import('@/views/ItemCategories.vue'),
            meta: {title: '物品分类', icon: 'Collection'},
        },
        {
            path: '/itemLocations',
            name: 'itemLocations',
            component: () => import('@/views/ItemLocations.vue'),
            meta: {title: '存放位置', icon: 'Location'},
        },
    ],
}

registerPlugin(itemsPlugin)
