<script setup lang="ts">
import {onMounted, ref, watch} from 'vue'
import {addDays, format, subDays} from 'date-fns'
import VChart from 'vue-echarts'
import '../plugins/echarts'
import {theme} from '@/theme'
import {type CappedItem, capTooltip, capValues, capYAxis, MAX_VISUAL, themeColors,} from './chartHelpers'

// 每日待办任务数的直方图：过期（早于今天）标红，未过期标琥珀
const chartOption = ref({})

const startDate = ref(format(subDays(new Date(), 6), 'yyyy-MM-dd'))
const endDate = ref(format(addDays(new Date(), 39), 'yyyy-MM-dd'))

async function fetchStats() {
  try {
    const params = new URLSearchParams({start: startDate.value, end: endDate.value})
    const res = await fetch(`/api/tasks/activeStats?${params.toString()}`)
    const data: { date: string; count: number }[] = await res.json()

    const today = new Date().toISOString().slice(0, 10)
    const capped: CappedItem[] = capValues(data.map(d => d.count))
    const colors = themeColors()

    chartOption.value = {
      animation: false,
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
  }
}

onMounted(fetchStats)

// 起止日期变化时重新拉取
watch([startDate, endDate], fetchStats)
// 主题切换时重新拉取，使图表随主题取色
watch(theme, fetchStats)
</script>

<template>
  <v-chart :option="chartOption" class="chart-wrapper"/>
</template>

<style scoped>
.chart-wrapper {
  height: 22vh;
  min-height: 180px;
}
</style>
