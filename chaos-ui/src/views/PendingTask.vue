<script setup lang="ts">
// 待办任务（任务收件箱）：以通用 DataTable 驱动只读列表展示（对齐「标准数据」基线），
// 但本视图不是 CRUD 资源，操作是 完成/取消/延期/复习/预览/跳转，
// 且列表为 tasks JOIN task_plans 的连表查询（后端 GetPendingTasks 自定义 handler）。
// 故仅复用 DataTable 的展示层（columns + #toolbar/#actions/#field 插槽），
// 业务动作、计划树筛选、提前查询、评分/延期弹窗、轮询刷新、逾期高亮等定制能力全部保留在此页。
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {batchPostponeTasks, sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import {format as formatDate, parseISO} from 'date-fns'
import {CircleClose} from '@element-plus/icons-vue'
import DataTable from '@/components/DataTable.vue'
import type {DataTableColumn, DataTableApiParams, DataTableApiResult} from '@/components/dataTable/types'
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

const dataTableRef = ref<any>(null)

// 提前查询开关 + 计划树筛选（本页自定义，不走 DataTable 标准 search）
const earlyMode = ref(true)
const selectedTasks = ref<PendingTask[]>([])

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

function onPlanNodeClick(node: PlanTreeNode) {
  filterPlanId.value = node.value
  filterPlanLabel.value = node.label
  planPopoverVisible.value = false
}

function clearPlanFilter() {
  filterPlanId.value = null
  filterPlanLabel.value = ''
}

// 搜索栏「重置」：清空自定义筛选（提前查询/计划树），恢复初始无过滤状态
function onSearchReset() {
  earlyMode.value = true
  filterPlanId.value = null
  filterPlanLabel.value = ''
}

function isPostponable(row: PendingTask): boolean {
  return row.PlanType === 'todo' || row.PlanType === 'interval'
}

// DataTable 取数：把分页 + 本页自定义 early/planId + 排序拼成 query，
// 对齐后端基线列表契约（page/size/sort/order → { items, total, page, size }）。
async function fetchPending(params: DataTableApiParams): Promise<DataTableApiResult> {
  const query: Record<string, any> = {
    early: earlyMode.value ? 1 : 0,
    page: params.page,
    size: params.pageSize,
  }
  if (params.search?.PlanName) query.name = params.search.PlanName
  if (filterPlanId.value) query.planId = filterPlanId.value
  if (params.sort?.field) {
    // 仅允许已知排序列；其余忽略（后端白名单二次校验）
    const map: Record<string, string> = {StartedAt: 'started_at'}
    const col = map[params.sort.field]
    if (col) {
      query.sort = col
      query.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
    }
  }
  const result = await sendMessage('tasks/pending', 'GET', query)
  return {rows: (result?.items ?? []) as PendingTask[], total: result?.total ?? 0}
}

function onSelectionChange(rows: any[]) {
  selectedTasks.value = rows as PendingTask[]
}

// 逾期行高亮：StartedAt 已到即标记（与原 cellStyle 行为一致，整行背景提示）
function rowClassName({row, rowIndex,}: { row: PendingTask, rowIndex: number }): string {
  if (row.StartedAt && Date.now() > parseISO(row.StartedAt).getTime()) {
    return 'pending-overdue-row'
  }
  return ''
}

// 列定义：仅声明展示列，操作列交由 #actions 插槽；不启用标准 CRUD。
const columns: DataTableColumn[] = [
  {field: 'PlanType', title: '类型', width: 90},
  {field: 'PlanName', title: '任务名称', minWidth: 200, searchable: true, search: {placeholder: '请输入任务名称'}},
  {field: 'ContentSize', title: '字数', width: 80},
  {field: 'FsrsReps', title: '复习次数', width: 90},
  {field: 'Status', title: '状态', width: 90},
  {field: 'StartedAt', title: '开始时间', width: 120, sortable: true},
  {field: '__actions', title: '操作', width: 240, type: 'actions', fixed: 'right'},
]

// ── 业务操作（保持原行为） ──────────────────────────────────────────────
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
      await sendMessage(`tasks/${ratingTargetTask.value.ID}/complete`, 'PATCH', {rating})
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

function postponeTask(task: PendingTask) {
  postponeTargetTask.value = task
  postponeDays.value = 1
  showPostponeDialog.value = true
}

function batchPostpone() {
  if (selectedTasks.value.length === 0) return
  postponeTargetTask.value = null
  postponeDays.value = 1
  showPostponeDialog.value = true
}

const showRatingDialog = ref(false)
const submittingRating = ref(false)
const ratingTargetTask = ref<PendingTask | null>(null)
const ratingValue = ref<number | null>(3)

const showPostponeDialog = ref(false)
const postponeTargetTask = ref<PendingTask | null>(null)
const postponeDays = ref<number>(1)
const postponePresets = [1, 3, 7]

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
      await sendMessage(`tasks/${postponeTargetTask.value.ID}/postpone`, 'PATCH', {days: postponeDays.value})
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

function reload() {
  dataTableRef.value?.refresh()
}

let pendingTimer: ReturnType<typeof setInterval>
let stopVersionWatch: () => void

onMounted(() => {
  loadPlanTree()
  pendingTimer = setInterval(() => reload(), 30000)
  // 订阅全局刷新信号：其它实例（如任务表格）操作后本实例实时同步
  stopVersionWatch = watch(pendingTasksVersion, () => reload())
})

onUnmounted(() => {
  clearInterval(pendingTimer)
  stopVersionWatch?.()
})

defineExpose({refresh: reload})
</script>

<template>
  <div class="pending-tasks-wrapper">
    <DataTable
      ref="dataTableRef"
      :columns="columns"
      :api="fetchPending"
      :selection="true"
      :selectable="isPostponable"
      :reserve-selection="true"
      :row-class-name="rowClassName"
      row-key="ID"
      title="待办任务"
      @selection-change="onSelectionChange"
      @reset="onSearchReset"
    >
      <!-- 搜索栏（第一行）：提前查询开关 + 任务计划筛选 + 任务名称（DataTable 自动生成输入框与 查询/重置） -->
      <template #search-extra>
        <el-form-item label="提前查询">
          <el-switch v-model="earlyMode" />
        </el-form-item>
        <el-form-item label="任务计划">
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
                size="small"
                class="plan-filter"
                placeholder="按任务计划筛选"
              >
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
        </el-form-item>
      </template>

      <!-- 工具栏（第二行，靠左）：批量延期 -->
      <template #toolbar>
        <el-button
          type="primary"
          size="small"
          :disabled="selectedTasks.length === 0"
          @click="batchPostpone"
        >批量延期
        </el-button>
        <span v-if="selectedTasks.length" class="selected-count">已选 {{ selectedTasks.length }} 项</span>
      </template>

      <!-- 类型：标签 -->
      <template #PlanType="{ row }">
        <el-tag size="small" :type="PLAN_TYPE_MAP[row.PlanType]?.type || 'info'">
          {{ PLAN_TYPE_MAP[row.PlanType]?.text || row.PlanType }}
        </el-tag>
      </template>

      <!-- 字数 -->
      <template #ContentSize="{ row }">
        <span v-if="row.ContentSize > 0">{{ row.ContentSize.toLocaleString() }}</span>
        <span v-else class="text-secondary">-</span>
      </template>

      <!-- 复习次数 -->
      <template #FsrsReps="{ row }">
        <span v-if="row.PlanType === 'interval'">{{ row.FsrsReps }}</span>
        <span v-else class="text-secondary">-</span>
      </template>

      <!-- 状态 -->
      <template #Status="{ row }">
        <el-tag v-if="row.IsOverdue" size="small" type="danger">已逾期</el-tag>
        <el-tag v-else size="small" type="primary">待处理</el-tag>
      </template>

      <!-- 开始时间 -->
      <template #StartedAt="{ row }">
        {{ formatTime(row.StartedAt) }}
      </template>

      <!-- 行内操作 -->
      <template #actions="{ row }">
        <div class="op-actions">
          <el-button v-if="row.RawLink" size="small" type="info" text @click="openPreview(row)">预览</el-button>
          <el-button v-if="row.RawLink && row.FsrsReps > 0" size="small" type="warning" text @click="openReview(row)">复习</el-button>
          <el-button v-if="row.Link" size="small" type="primary" text @click="openLink(row.Link)">跳转</el-button>
          <el-button v-if="row.PlanType === 'cron'" size="small" type="danger" text @click="cancelTask(row)">取消</el-button>
          <el-button v-if="row.PlanType === 'todo' || row.PlanType === 'interval'" size="small" text @click="postponeTask(row)">延期</el-button>
          <el-button size="small" type="success" text @click="completeTask(row)">完成</el-button>
        </div>
      </template>
    </DataTable>

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

.selected-count {
  color: var(--el-text-color-secondary);
  font-size: 0.85rem;
  margin-left: var(--space-sm);
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

<style>
/* 逾期行高亮（跨 scoped，因 el-table 行 class 生成在组件根外）。
   背景须落在单元格 td 上：el-table 的 td 自带背景色，只改 tr 会被盖住。 */
.el-table .pending-overdue-row > td.el-table__cell {
  background-color: rgba(245, 108, 108, 0.12);
}

/* 悬停时保持逾期底色（el-table 默认 hover 背景优先级更高，需显式覆盖） */
.el-table .pending-overdue-row:hover > td.el-table__cell {
  background-color: rgba(245, 108, 108, 0.2);
}
</style>
