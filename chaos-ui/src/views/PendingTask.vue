<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {batchPostponeTasks, sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import {format as formatDate, parseISO} from 'date-fns'
import {CircleClose} from '@element-plus/icons-vue'
import {pendingTasksVersion, refreshPendingTasks} from '@/utils/pendingTasksStore'
import {openCenterPanel} from '@/utils/centerPanel'
import RatingDialog from '@/components/RatingDialog.vue'
import {PLAN_TYPE_MAP} from '@/constants'

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
const earlyMode = ref(true)
const selectedTasks = ref<PendingTask[]>([])

// 服务端分页：配合后端 tasks/pending 的 { items, total, page, size } 响应
const page = ref(1)
const size = ref(10)
const total = ref(0)

// 任务计划筛选：点击展开计划树（最多两层），选中后按该计划及其子孙计划过滤
interface PlanTreeNode {
  value: number
  label: string
  children?: PlanTreeNode[]
}

const planTree = ref<PlanTreeNode[]>([])
const filterPlanId = ref<number | null>(null)
const filterPlanLabel = ref('')
const planPopoverVisible = ref(false)

/** 裁剪为两层：根计划 + 其直接子计划（更深层级由后端在筛选时一并包含） */
function toPlanTreeOptions(nodes: any[]): PlanTreeNode[] {
  return (nodes || []).map(node => {
    const children: PlanTreeNode[] = (node.Children || []).map((child: any) => ({
      value: child.ID,
      label: child.Name,
    }))
    return children.length > 0
        ? {value: node.ID, label: node.Name, children}
        : {value: node.ID, label: node.Name}
  })
}

async function loadPlanTree() {
  try {
    const result = await sendMessage('taskPlans/tree', 'GET')
    if (Array.isArray(result)) {
      planTree.value = toPlanTreeOptions(result)
    }
  } catch (e) {
    // 计划树加载失败不阻塞待办列表，仅退化为「无法按计划筛选」
    console.error(e)
  }
}

function handlePlanFilterChange() {
  // 切换筛选后数据集合变化，回到第 1 页避免落到空页
  page.value = 1
  loadPendingTasks()
}

// 点击计划树节点（含第一层根计划）：选中并关闭面板
function onPlanNodeClick(node: PlanTreeNode) {
  filterPlanId.value = node.value
  filterPlanLabel.value = node.label
  planPopoverVisible.value = false
  handlePlanFilterChange()
}

// 清空筛选：恢复全量待办
function clearPlanFilter() {
  filterPlanId.value = null
  filterPlanLabel.value = ''
  handlePlanFilterChange()
}

function handlePageChange(p: number) {
  page.value = p
  loadPendingTasks()
}

function handleSizeChange(s: number) {
  size.value = s
  page.value = 1
  loadPendingTasks()
}

function handleEarlyModeChange() {
  // 切换「提前查询」后数据集合变化，回到第 1 页避免落到空页
  page.value = 1
  loadPendingTasks()
}

function handleSelectionChange(rows: PendingTask[]) {
  selectedTasks.value = rows
}

function isPostponable(row: PendingTask): boolean {
  return row.PlanType === 'todo' || row.PlanType === 'interval'
}

const showRatingDialog = ref(false)
const submittingRating = ref(false)
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

function formatTime(timeStr: string | null) {
  if (!timeStr) return ''
  try {
    return formatDate(new Date(timeStr), 'MM-dd HH:mm')
  } catch {
    return ''
  }
}

function openLink(link: string) {
  window.open(link, '_blank')
}

async function loadPendingTasks() {
  try {
    const planParam = filterPlanId.value ? `&planId=${filterPlanId.value}` : ''
    const url = `tasks/pending?early=${earlyMode.value ? 1 : 0}&page=${page.value}&size=${size.value}${planParam}`
    const result = await sendMessage(url, 'GET')
    if (result && Array.isArray(result.items)) {
      pendingTasks.value = result.items
      total.value = result.total
      // 当前页被取空且非首页（通常是完成/取消后数据变少），回退一页再拉取
      if (pendingTasks.value.length === 0 && page.value > 1) {
        page.value -= 1
        return loadPendingTasks()
      }
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

async function submitRatingDialog(rating: number) {
  submittingRating.value = true
  try {
    if (ratingTargetTask.value) {
      await sendMessage(`tasks/${ratingTargetTask.value.ID}/complete`, 'PATCH', {
        rating,
      })
      ElMessage.success('任务已完成')
    }
    showRatingDialog.value = false
    ratingTargetTask.value = null
    refreshPendingTasks()

  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  } finally {
    submittingRating.value = false
  }
}

async function postponeTask(task: PendingTask) {
  postponeTargetTask.value = task
  postponeDays.value = 1
  showPostponeDialog.value = true
}

async function batchPostpone() {
  if (selectedTasks.value.length === 0) return
  postponeTargetTask.value = null
  postponeDays.value = 1
  showPostponeDialog.value = true
}

const postponeDialogTitle = computed(() => {
  if (postponeTargetTask.value) {
    return `延期任务 — ${postponeTargetTask.value.PlanName}`
  }
  return `批量延期（已选 ${selectedTasks.value.length} 个）`
})

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
    } else if (selectedTasks.value.length > 0) {
      const ids = selectedTasks.value.map(t => t.ID)
      const res = await batchPostponeTasks(ids, postponeDays.value)
      const msg = res?.message || `已延期 ${postponeDays.value} 天`
      if (res && res.skipped > 0) {
        ElMessage.warning(msg)
      } else {
        ElMessage.success(msg)
      }
    }
    showPostponeDialog.value = false
    postponeTargetTask.value = null
    selectedTasks.value = []
    refreshPendingTasks()

  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

function cellStyle({row}: { row: PendingTask }) {
  if (row.StartedAt && Date.now() > parseISO(row.StartedAt).getTime()) {
    return {backgroundColor: 'rgba(245, 108, 108, 0.12)'}
  }
  return {}
}


let pendingTimer: ReturnType<typeof setInterval>
let stopVersionWatch: () => void

onMounted(() => {
  loadPlanTree()
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
          @change="handleEarlyModeChange"
      />
      <el-popover
          v-model:visible="planPopoverVisible"
          placement="bottom-start"
          :width="260"
          trigger="click"
      >
        <template #reference>
          <el-input
              :model-value="filterPlanLabel"
              readonly
              class="plan-filter"
              placeholder="按任务计划筛选"
          >
            <!-- readonly 输入框不展示 el-input 自带的 clearable 图标，此处自绘清除按钮 -->
            <template #suffix>
              <el-icon
                  v-if="filterPlanId"
                  class="plan-filter-clear"
                  title="清除筛选"
                  @click.stop.prevent="clearPlanFilter"
              >
                <CircleClose/>
              </el-icon>
            </template>
          </el-input>
        </template>
        <el-tree
            :data="planTree"
            node-key="value"
            :current-node-key="filterPlanId ?? undefined"
            :expand-on-click-node="false"
            highlight-current
            @node-click="onPlanNodeClick"
        />
      </el-popover>
      <div class="toolbar-actions">
        <span v-if="selectedTasks.length" class="selected-count">已选 {{ selectedTasks.length }} 项</span>
        <el-button
            type="primary"
            :disabled="selectedTasks.length === 0"
            @click="batchPostpone"
        >批量延期
        </el-button>
      </div>
    </div>

    <el-empty
        v-if="pendingTasks.length === 0"
        :description="filterPlanId ? '该计划下暂无待办' : '暂无待办'"
        class="pending-empty"
    />

    <div v-else>
      <el-table :data="pendingTasks" border stripe class="mb-sm" row-key="ID" @selection-change="handleSelectionChange" :cell-style="cellStyle">
        <el-table-column type="selection" width="48" :selectable="isPostponable" reserve-selection/>
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="PLAN_TYPE_MAP[row.PlanType]?.type || 'info'">
              {{ PLAN_TYPE_MAP[row.PlanType]?.text || row.PlanType }}
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

      <div class="pager">
        <el-pagination :current-page="page" :page-size="size" :total="total"
                       :page-sizes="[15, 30, 100, 1000]"
                       layout="total, sizes, prev, pager, next, jumper"
                       @current-change="handlePageChange"
                       @size-change="handleSizeChange"
        />
      </div>
    </div>

    <RatingDialog
        v-model="showRatingDialog"
        v-model:rating="ratingValue"
        title="完成间隔任务"
        :target-name="ratingTargetTask?.PlanName || ''"
        :loading="submittingRating"
        @submit="submitRatingDialog"
    />

    <el-dialog v-model="showPostponeDialog" :title="postponeDialogTitle" width="26.25rem">
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

.plan-filter {
  width: 14rem;
  margin-left: var(--space-sm);
}

.plan-filter-clear {
  cursor: pointer;
  color: var(--el-text-color-placeholder);
}

.plan-filter-clear:hover {
  color: var(--el-text-color-secondary);
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-left: auto;
}

.selected-count {
  color: var(--el-text-color-secondary);
  font-size: 0.85rem;
}

.pending-empty {
  margin-top: var(--space-2xl);
}

.pager {
  margin-top: var(--space-sm);
  display: flex;
  justify-content: flex-end;
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