import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'

// 按需注册 ECharts 组件，减小打包体积。
// 目前看板只用折线图（line）与直方图（bar）；新增图表类型时在此补充注册。
use([
  CanvasRenderer,
  LineChart,
  BarChart,
  TooltipComponent,
  GridComponent,
])
