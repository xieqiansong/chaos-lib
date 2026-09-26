<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import ContributionPanel from '../components/ContributionPanel.vue'
import DailyStatsChart from '../components/DailyStatsChart.vue'
import ActiveStatsChart from '../components/ActiveStatsChart.vue'

const router = useRouter()

const balance = ref<string>('')
const loading = ref(false)
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
  fetchContribution()
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
      <el-col :span="12">
        <div class="chart-section">
          <DailyStatsChart/>
        </div>
      </el-col>
      <el-col :span="12">
        <div class="chart-section">
          <ActiveStatsChart/>
        </div>
      </el-col>
    </el-row>
    <el-row :gutter="16" class="mb-lg">
      <el-col :span="12">
        <div class="chart-section">
          <div v-if="contributionLoading" class="contrib-placeholder">加载中...</div>
          <ContributionPanel v-else :root-name="contributionRoot" :items="contributionItems"/>
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
  margin-bottom: var(--space-lg);
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

.contrib-placeholder {
  height: 144px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}
</style>
