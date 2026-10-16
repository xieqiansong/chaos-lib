// mqttsync 前端插件：路由与菜单 meta；后端 internal/platform/mqttsync/plugin.go 已对应登记。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const mqttsyncPlugin: Plugin = {
  meta: { id: 'mqttsync', name: 'MQTT同步', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/mqttSync',
      name: 'mqttSync',
      component: () => import('@/views/MqttSync.vue'),
      meta: { title: 'MQTT同步', icon: 'Connection' },
    },
  ],
}

registerPlugin(mqttsyncPlugin)
