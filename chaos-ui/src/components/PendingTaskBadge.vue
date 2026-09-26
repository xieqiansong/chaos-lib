<script setup lang="ts">
import {onMounted, onUnmounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {Clock} from '@element-plus/icons-vue'
import {sendMessage} from '@/utils/api'
import {pendingTasksVersion} from '@/utils/pendingTasksStore'

// 页头「待办任务」入口：红色数量角标（同消息未读样式），点击进入待办任务页。
// 数量取 /tasks/pending 分页响应的 total（与待办页同一口径，不含「提前查询」的未到点任务），
// 30s 轮询兜底；完成 / 取消 / 延期 / 评分后由全局刷新信号即时同步，无需等下一轮轮询。
const POLL_INTERVAL = 30000
const MAX_DISPLAY = 99

const router = useRouter()
const count = ref(0)

let timer: ReturnType<typeof setInterval>
let stopVersionWatch: () => void

async function loadCount() {
  try {
    const result = await sendMessage('tasks/pending', 'GET')
    // 后端返回分页结构 { items, total, page, size }，角标取 total 作为待办总数
    if (result && typeof result.total === 'number') {
      count.value = result.total
    }
  } catch (e) {
    console.error(e)
  }
}

function openPendingTasks() {
  router.push('/pendingTask')
}

onMounted(() => {
  loadCount()
  timer = setInterval(loadCount, POLL_INTERVAL)
  stopVersionWatch = watch(pendingTasksVersion, () => loadCount())
})

onUnmounted(() => {
  clearInterval(timer)
  stopVersionWatch?.()
})
</script>

<template>
  <button
      class="pending-badge-btn"
      :title="count > 0 ? `待办任务（${count}）` : '待办任务'"
      @click="openPendingTasks"
  >
    <el-icon :size="16">
      <Clock/>
    </el-icon>
    <span v-if="count > 0" class="pending-badge">{{ count > MAX_DISPLAY ? `${MAX_DISPLAY}+` : count }}</span>
  </button>
</template>

<style scoped>
/* 顶栏「待办任务」按钮：与 ⌘K / 主题 / 横屏按钮同尺寸同风格（PX 固定，不随根字号缩放），
   按钮间距由 App.vue 的 .header-actions gap 统一控制，不再单独留 margin */
.pending-badge-btn {
  position: relative;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--term-inner);
  border: 1px solid var(--term-border);
  color: var(--term-green-faint);
  font-family: inherit;
  font-size: 13PX;
  padding: 5PX 12PX;
  cursor: pointer;
  border-radius: 4PX;
}

.pending-badge-btn:hover {
  color: var(--term-green);
  border-color: var(--term-green-dim);
}

/* 未读式红色角标：仅有待办时出现，位置在图标右上角（不超出容器，避免被外壳 overflow 裁掉） */
.pending-badge {
  position: absolute;
  top: -5px;
  right: -5px;
  min-width: 15px;
  height: 15px;
  padding: 0 3px;
  box-sizing: border-box;
  border-radius: 8px;
  background: var(--term-red);
  color: #fff;
  font-size: 10px;
  font-weight: 600;
  line-height: 15px;
  text-align: center;
  pointer-events: none;
}
</style>
