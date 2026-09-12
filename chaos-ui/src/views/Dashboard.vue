<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {theme} from '../theme'
import ContributionPanel from '../components/ContributionPanel.vue'

const router = useRouter()

const balance = ref<string>('')
const loading = ref(false)
const chartLoading = ref(false)
const chartOption = ref({})
const activeChartLoading = ref(false)
const activeChartOption = ref({})
const isBalanceNum = computed(() => !isNaN(Number(balance.value)) && balance.value !== '')

// GitHub 风格贡献图：近一年每日完成任务数（按「每日任务」下的子任务分别统计，可切换）
const contributionLoading = ref(false)
const contributionRoot = ref('每日任务')
const contributionItems = ref<{
  id: number
  name: string
  total: number
  days: { date: string; count: number }[]
}[]>([])

const MAX_VISUAL = 99
type CappedItem = { visual: number; raw: number }

// 从当前主题读取主题色，避免图表内游离硬编码（随主题切换实时取色）
const cssVar = (name: string) =>
    getComputedStyle(document.documentElement).getPropertyValue(name).trim()

function themeColors() {
  return {
    green: cssVar('--term-green'),
    red: cssVar('--term-red'),
    amber: cssVar('--term-amber'),
    areaTop: cssVar('--term-area-top'),
    areaBottom: cssVar('--term-area-bottom'),
  }
}

function capValues(counts: number[]): CappedItem[] {
  return counts.map(c => ({visual: Math.min(c, MAX_VISUAL), raw: c}))
}

function capYAxis() {
  return {max: MAX_VISUAL, axisLabel: {formatter: (v: number) => (v >= MAX_VISUAL ? `${MAX_VISUAL}+` : v)}}
}

function capTooltip(prefix: string, capped: CappedItem[]) {
  return (params: any) => {
    const d = params[0]
    const item = capped[d.dataIndex]
    const extra = item && item.raw > MAX_VISUAL ? `（实际: ${item.raw}）` : ''
    return `${d.name}<br/>${d.marker} ${prefix}: ${d.value}${extra}`
  }
}

async function fetchBalance() {
  loading.value = true
  try {
    const res = await fetch('/api/balance/deepseek')
    const data = await res.json()
    if (!res.ok) {
      balance.value = `查询失败: ${data.error || '未知错误'}`
      return
    }
    const total = data.balance_infos?.[0]?.total_balance
    balance.value = total || '未获取到余额'
  } catch (e: any) {
    balance.value = `请求失败: ${e.message}`
  } finally {
    loading.value = false
  }
}

async function fetchDailyStats() {
  chartLoading.value = true
  try {
    const res = await fetch('/api/tasks/dailyStats')
    const data: { date: string; count: number }[] = await res.json()
    const capped = capValues(data.map(d => d.count))
    const colors = themeColors()

    chartOption.value = {
      tooltip: {trigger: 'axis', formatter: capTooltip('完成', capped)},
      xAxis: {
        type: 'category',
        data: data.map(d => d.date.slice(5)),
        axisLabel: {rotate: 90, interval: 0, fontSize: 11},
      },
      yAxis: Object.assign(
          {type: 'value', minInterval: 1, name: '完成数'},
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
      grid: {left: '4%', right: '2%', top: '16%', bottom: '8%'},
    }
  } catch (e: any) {
    console.error('获取任务统计失败:', e)
  } finally {
    chartLoading.value = false
  }
}

async function fetchActiveStats() {
  activeChartLoading.value = true
  try {
    const res = await fetch('/api/tasks/activeStats')
    const data: { date: string; count: number }[] = await res.json()

    const today = new Date().toISOString().slice(0, 10)
    const capped = capValues(data.map(d => d.count))
    const colors = themeColors()

    activeChartOption.value = {
      tooltip: {
        trigger: 'axis',
        formatter: capTooltip('待办', capped),
      },
      xAxis: {
        type: 'category',
        data: data.map(d => d.date.slice(5)),
        axisLabel: {rotate: 90, interval: 0, fontSize: 11},
      },
      yAxis: Object.assign(
          {type: 'value', minInterval: 1, name: '任务数'},
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
      grid: {left: '4%', right: '2%', top: '16%', bottom: '8%'},
    }
  } catch (e: any) {
    console.error('获取待办任务统计失败:', e)
  } finally {
    activeChartLoading.value = false
  }
}

async function fetchContribution() {
  contributionLoading.value = true
  try {
    const res = await fetch('/api/tasks/contributionStats')
    const data = await res.json()
    contributionRoot.value = data.rootName || '每日任务'
    contributionItems.value = data.items || []
  } catch (e: any) {
    console.error('获取贡献统计失败:', e)
  } finally {
    contributionLoading.value = false
  }
}

function goBoard() {
  router.push('/board')
}

onMounted(() => {
  fetchBalance()
  fetchDailyStats()
  fetchActiveStats()
  fetchContribution()
})

watch(theme, () => {
  // 主题切换时重新拉取统计，使图表随主题取色
  fetchDailyStats()
  fetchActiveStats()
})
</script>

<template>
  <div class="dashboard">
    <el-row>
      <el-col :span="24">
        <div class="api-balance">
          <span v-if="loading">查询中...</span>
          <span v-else-if="isBalanceNum">
            DeepSeek API 剩余
            <span class="balance-amount">{{ balance }}</span>
            CNY
          </span>
          <span v-else>{{ balance }}</span>
          <el-button class="clock-jump-btn" type="primary" plain size="small" @click="goBoard">
            全屏时钟
          </el-button>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mb-lg">
      <el-col :span="24">
        <div class="chart-section">
          <div class="section-toolbar">
            <span class="text-primary text-base section-title">任务贡献</span>
          </div>
          <div v-if="contributionLoading" class="contrib-placeholder">加载中...</div>
          <ContributionPanel v-else :root-name="contributionRoot" :items="contributionItems"/>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="12">
        <div class="chart-section">
          <div v-if="chartLoading" class="chart-placeholder">加载中...</div>
          <v-chart v-else ref="chartRef" :option="chartOption" class="chart-wrapper"/>
        </div>
      </el-col>
      <el-col :span="12">
        <div class="chart-section">
          <div v-if="activeChartLoading" class="chart-placeholder">加载中...</div>
          <v-chart v-else :option="activeChartOption" class="chart-wrapper"/>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 100%;
}

.api-balance {
  margin-bottom: var(--space-2xl);
  font-size: var(--el-font-size-base);
}

.clock-jump-btn {
  margin-left: var(--space-lg);
}

.balance-amount {
  color: var(--el-color-success);
  font-weight: bold;
}

.chart-section {
  background: var(--el-bg-color);
  border-radius: var(--el-border-radius-base);
  padding: var(--space-lg);
  border: 1px solid var(--el-border-color-lighter);
}

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

.contrib-placeholder {
  height: 9rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}
</style>
