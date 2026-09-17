<script setup lang="ts">
import {onMounted, onUnmounted, ref, watch} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import {format} from 'date-fns'
import {pendingTasksVersion, refreshPendingTasks} from '@/utils/pendingTasksStore'
import {openCenterPanel} from '@/utils/centerPanel'

interface PendingTask {
  ID: number
  PlanID: number
  Status: string
  StartedAt: string | null
  CompletedAt: string | null
  Deadline: string | null
  Remark: string | null
  Link: string | null
  RawLink: string | null
  ContentSize: number
  CreatedAt: string
  PlanName: string
  PlanType: string
  FsrsReps: number
  IsOverdue: boolean
}

const pendingTasks = ref<PendingTask[]>([])
const earlyMode = ref(false)

const showRatingDialog = ref(false)
const ratingTargetTask = ref<PendingTask | null>(null)
const ratingValue = ref<number | null>(3)

const showPostponeDialog = ref(false)
const postponeTargetTask = ref<PendingTask | null>(null)
const postponeDays = ref<number>(1)
const postponePresets = [1, 3, 7]

function openReview(task: PendingTask) {
  openCenterPanel('review', task.PlanID, task.PlanName)
}

function openPreview(task: PendingTask) {
  if (!task.RawLink) return
  openCenterPanel('preview', task.PlanID, task.PlanName)
}

const ratingOptions = [
  {value: 1, label: 'Again（忘记了）', type: 'danger'},
  {value: 2, label: 'Hard（记得但困难）', type: 'warning'},
  {value: 3, label: 'Good（正常记得）', type: 'primary'},
  {value: 4, label: 'Easy（太简单）', type: 'success'},
]

const planTypeMap: Record<string, { text: string, type: string }> = {
  todo: {text: '待办', type: 'primary'},
  cron: {text: '周期', type: 'success'},
  interval: {text: '间隔', type: 'warning'},
}

function formatTime(timeStr: string | null) {
  if (!timeStr) return ''
  try {
    return format(new Date(timeStr), 'MM-dd HH:mm')
  } catch {
    return ''
  }
}

function openLink(link: string) {
  window.open(link, '_blank')
}

async function loadPendingTasks() {
  try {
    const url = earlyMode.value ? 'tasks/pending?early=1' : 'tasks/pending'
    const result = await sendMessage(url, 'GET')
    if (Array.isArray(result)) {
      pendingTasks.value = result
    }
  } catch (e) {
    console.error(e)
  }
}

async function completeTask(task: PendingTask) {
  if (task.PlanType === 'interval') {
    ratingTargetTask.value = task
    ratingValue.value = 3
    showRatingDialog.value = true
    return
  }
  try {
    await ElMessageBox.confirm('确认完成此任务？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info',
    })
    await sendMessage(`tasks/${task.ID}/complete`, 'PATCH', {})
    refreshPendingTasks()

    ElMessage.success('任务已完成')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
  }
}

async function cancelTask(task: PendingTask) {
  try {
    await ElMessageBox.confirm('确认取消此周期任务？取消后本次任务将不再提醒。', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await sendMessage(`tasks/${task.ID}/cancel`, 'PATCH', {})
    refreshPendingTasks()

    ElMessage.success('任务已取消')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
  }
}

async function submitRatingDialog() {
  if (ratingValue.value === null) {
    ElMessage.error('请选择评分')
    return
  }
  try {
    if (ratingTargetTask.value) {
      await sendMessage(`tasks/${ratingTargetTask.value.ID}/complete`, 'PATCH', {
        rating: ratingValue.value,
      })
      ElMessage.success('任务已完成')
    }
    showRatingDialog.value = false
    ratingTargetTask.value = null
    refreshPendingTasks()

  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

async function postponeTask(task: PendingTask) {
  postponeTargetTask.value = task
  postponeDays.value = 1
  showPostponeDialog.value = true
}

async function submitPostponeDialog() {
  if (postponeDays.value <= 0) {
    ElMessage.error('延期天数必须大于0')
    return
  }
  try {
    if (postponeTargetTask.value) {
      await sendMessage(`tasks/${postponeTargetTask.value.ID}/postpone`, 'PATCH', {
        days: postponeDays.value,
      })
      ElMessage.success(`已延期 ${postponeDays.value} 天`)
    }
    showPostponeDialog.value = false
    postponeTargetTask.value = null
    refreshPendingTasks()

  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

let pendingTimer: ReturnType<typeof setInterval>
let stopVersionWatch: () => void

onMounted(() => {
  loadPendingTasks()
  pendingTimer = setInterval(() => {
    loadPendingTasks()
  }, 30000)
  // 订阅全局刷新信号：其它实例（如任务表格）操作后本实例实时同步
  stopVersionWatch = watch(pendingTasksVersion, () => {
    loadPendingTasks()
  })
})

onUnmounted(() => {
  clearInterval(pendingTimer)
  stopVersionWatch?.()
})

defineExpose({loadPendingTasks})
</script>

<template>
  <div class="pending-tasks-wrapper">
    <div class="pending-toolbar">
      <el-switch
          v-model="earlyMode"
          active-text="提前查询"
          @change="loadPendingTasks"
      />
    </div>

    <el-empty v-if="pendingTasks.length === 0" description="暂无待办" class="pending-empty"/>

    <div v-else>
      <el-table :data="pendingTasks" border stripe class="mb-sm">
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="planTypeMap[row.PlanType]?.type || 'info'">
              {{ planTypeMap[row.PlanType]?.text || row.PlanType }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="PlanName" label="任务名称" min-width="200"/>
        <el-table-column label="字数" width="80">
          <template #default="{ row }">
            <span v-if="row.ContentSize > 0">{{ row.ContentSize.toLocaleString() }}</span>
            <span v-else class="text-secondary">-</span>
          </template>
        </el-table-column>
        <el-table-column label="复习次数" width="90">
          <template #default="{ row }">
            <span v-if="row.PlanType === 'interval'">{{ row.FsrsReps }}</span>
            <span v-else class="text-secondary">-</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag v-if="row.IsOverdue" size="small" type="danger">已逾期</el-tag>
            <el-tag v-else size="small" type="primary">待处理</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="120">
          <template #default="{ row }">
            {{ formatTime(row.StartedAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <div class="op-actions">
            <el-button v-if="row.RawLink" size="small" type="info" text @click="openPreview(row)">预览</el-button>
            <el-button v-if="row.RawLink && row.FsrsReps > 0" size="small" type="warning" text @click="openReview(row)">复习</el-button>
            <el-button v-if="row.Link" size="small" type="primary" text @click="openLink(row.Link!)">跳转</el-button>
            <el-button v-if="row.PlanType === 'cron'" size="small" type="danger" text @click="cancelTask(row)">取消</el-button>
            <el-button v-if="row.PlanType === 'todo' || row.PlanType === 'interval'" size="small" text @click="postponeTask(row)">延期</el-button>
            <el-button size="small" type="success" text @click="completeTask(row)">完成</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
        v-model="showRatingDialog"
        :title="`完成间隔任务 — ${ratingTargetTask?.PlanName || ''}`"
        width="30rem"
    >
      <el-alert
          type="info"
          :closable="false"
          show-icon
          title="阅读场景：提示下次重读的紧迫度。Easy→已烂熟，Good→按节奏，Hard→值得回顾，Again→完全没印象。不影响学习难度，只影响下次出现时间。"
          class="mb-sm"
      />
      <div class="rating-group">
        <el-radio-group v-model="ratingValue">
          <el-radio v-for="opt in ratingOptions" :key="opt.value" :value="opt.value" :label="opt.value">
            {{ opt.label }}
          </el-radio>
        </el-radio-group>
      </div>
      <template #footer>
        <el-button @click="showRatingDialog = false; ratingTargetTask = null">取消</el-button>
        <el-button type="primary" @click="submitRatingDialog">确认</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showPostponeDialog"
        :title="`延期任务 — ${postponeTargetTask?.PlanName || ''}`"
        width="26.25rem"
    >
      <div class="postpone-content">
        <p class="text-secondary mb-sm">选择延期天数，任务开始时间将向后顺延。</p>
        <div class="postpone-presets">
          <el-button
              v-for="d in postponePresets"
              :key="d"
              :type="postponeDays === d ? 'primary' : 'default'"
              @click="postponeDays = d"
          >
            {{ d }} 天
          </el-button>
          <el-input-number
              v-model="postponeDays"
              :min="1"
              :max="365"
              placeholder="自定义"
              style="width: 7.5rem"
          />
        </div>
      </div>
      <template #footer>
        <el-button @click="showPostponeDialog = false; postponeTargetTask = null">取消</el-button>
        <el-button type="primary" @click="submitPostponeDialog">确认延期</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.pending-tasks-wrapper {
  height: 100%;
}

.pending-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: var(--space-sm);
}

.pending-empty {
  margin-top: var(--space-2xl);
}

.postpone-content {
  padding: var(--space-sm) 0;
}

.postpone-presets {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
}
</style>