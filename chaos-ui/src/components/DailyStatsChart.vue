<script setup lang="ts">
import {onMounted, ref, watch} from 'vue'
import {format} from 'date-fns'
import {theme} from '../theme'
import {
  MAX_VISUAL,
  capTooltip,
  capValues,
  capYAxis,
  themeColors,
  type CappedItem,
} from './chartHelpers'

// 近一年每日完成任务数的折线图（带面积渐变）
const loading = ref(false)
const chartOption = ref({})

async function fetchStats() {
  loading.value = true
  try {
    const res = await fetch('/api/tasks/dailyStats')
    const data: { date: string; count: number }[] = await res.json()
    const capped: CappedItem[] = capValues(data.map(d => d.count))
    const colors = themeColors()

    chartOption.value = {
      tooltip: {trigger: 'axis', formatter: capTooltip('完成', capped)},
      xAxis: {
        type: 'category',
        data: data.map(d => format(new Date(d.date), 'MM.dd')),
        axisLabel: {rotate: 90, interval: 0, fontSize: 11},
      },
      yAxis: Object.assign(
          {type: 'value', minInterval: 1},
          capYAxis(),
      ),
      series: [
        {
          name: '完成数',
          type: 'line',
          data: capped.map(c => c.visual),
          smooth: true,
          label: {
            show: true,
            position: 'top',
            formatter: (p: any) => {
              const item = capped[p.dataIndex]
              return item && item.raw > MAX_VISUAL ? `${MAX_VISUAL}+` : ''
            },
          },
          areaStyle: {
            color: {
              type: 'linear',
              x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                {offset: 0, color: colors.areaTop},
                {offset: 1, color: colors.areaBottom},
              ],
            },
          },
          lineStyle: {color: colors.green, width: 2},
          itemStyle: {color: colors.green},
        },
      ],
      grid: {left: '0', right: '0', top: '0', bottom: '0'},
    }
  } catch (e: any) {
    console.error('获取任务统计失败:', e)
  } finally {
    loading.value = false
  }
}

onMounted(fetchStats)

// 主题切换时重新拉取，使图表随主题取色
watch(theme, fetchStats)
</script>

<template>
  <div v-if="loading" class="chart-placeholder">加载中...</div>
  <v-chart v-else :option="chartOption" class="chart-wrapper"/>
</template>

<style scoped>
.chart-wrapper {
  height: 22vh;
  min-height: 180px;
}

.chart-placeholder {
  height: 22vh;
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}
</style>
