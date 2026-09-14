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

// 近一年每日待办任务数的直方图：过期（早于今天）标红，未过期标琥珀
const loading = ref(false)
const chartOption = ref({})

async function fetchStats() {
  loading.value = true
  try {
    const res = await fetch('/api/tasks/activeStats')
    const data: { date: string; count: number }[] = await res.json()

    const today = new Date().toISOString().slice(0, 10)
    const capped: CappedItem[] = capValues(data.map(d => d.count))
    const colors = themeColors()

    chartOption.value = {
      tooltip: {
        trigger: 'axis',
        formatter: capTooltip('待办', capped),
      },
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
          name: '待办',
          type: 'bar',
          data: capped.map((c, i) => ({
            value: c.visual,
            itemStyle: {
              color: data[i].date < today ? colors.red : colors.amber,
            },
          })),
          barMaxWidth: 20,
          label: {
            show: true,
            position: 'top',
            formatter: (p: any) => {
              const item = capped[p.dataIndex]
              return item && item.raw > MAX_VISUAL ? `${MAX_VISUAL}+` : ''
            },
          },
        },
      ],
      grid: {left: '0', right: '0', top: '0', bottom: '0'},
    }
  } catch (e: any) {
    console.error('获取待办任务统计失败:', e)
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
