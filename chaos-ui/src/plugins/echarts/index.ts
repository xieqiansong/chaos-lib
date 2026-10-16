// echarts 能力型插件：通过 install 钩子按需注册 ECharts 组件，减小打包体积。
// 与路由型插件（如 taskplan）不同，它不贡献路由，只贡献全局能力。
// 目前看板只用折线图（line）与直方图（bar）；新增图表类型时在此补充注册。
import { registerPlugin } from '../registry'
import type { Plugin } from '../types'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'

export const echartsPlugin: Plugin = {
  meta: {
    id: 'echarts',
    name: 'ECharts 按需注册',
    version: '1.0.0',
    dependencies: [],
  },
  install() {
    use([
      CanvasRenderer,
      LineChart,
      BarChart,
      TooltipComponent,
      GridComponent,
    ])
  },
}

// 自注册
registerPlugin(echartsPlugin)
