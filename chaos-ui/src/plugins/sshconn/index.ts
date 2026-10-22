// sshconn 前端插件：路由与菜单 meta；后端接口随 portfwd 模块挂载（ssh-conns 资源），
// 此处仅以独立插件对齐前后端插件 ID。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'

export const sshconnPlugin: Plugin = {
  meta: { id: 'sshconn', name: 'SSH连接', version: '1.0.0', dependencies: [] },
  routes: [
    {
      path: '/sshConn',
      name: 'sshConn',
      component: () => import('./SshConn.vue'),
      meta: { title: 'SSH连接', icon: 'Key' },
    },
  ],
}

registerPlugin(sshconnPlugin)
