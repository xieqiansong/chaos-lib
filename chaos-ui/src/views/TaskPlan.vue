<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import {openCenterPanel} from '@/utils/centerPanel'
import {refreshPendingTasks} from '@/utils/pendingTasksStore'
import {taskPlansVersion} from '@/utils/taskPlansStore'
import TaskPlanForm from '@/components/TaskPlanForm.vue'
import RatingDialog from '@/components/RatingDialog.vue'
import {PLAN_TYPE_MAP} from '@/constants'

const props = defineProps<{
  searchText: string
}>()

interface TaskPlan {
  ID: number
  ParentID: number | null
  Name: string
  Status: string
  PlanType: string
  CronExpr: string | null
  StartedAt: string | null
  Remark: string | null
  Link: string | null
  OrderNum: number | null
  Priority: number | null
  CreatedAt: string
  UpdatedAt: string
  IsSuspended: boolean
}

interface TaskPlanTree extends TaskPlan {
  Children: TaskPlanTree[]
  hasChildren?: boolean
}

const allPlans = ref<TaskPlanTree[]>([])
const fullTree = ref<TaskPlanTree[]>([])
const treeLoading = ref(false)
const error = ref('')

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showAddChildDialog = ref(false)
const editingPlan = ref<TaskPlan | null>(null)
const parentPlan = ref<TaskPlan | null>(null)

const showRatingDialog = ref(false)
const submittingRating = ref(false)
const ratingAction = ref<'start-plan' | 'complete-plan'>('complete-plan')
const ratingTargetPlan = ref<TaskPlan | null>(null)
const ratingValue = ref<number | null>(3)

const showPriorityDialog = ref(false)
const priorityTargetPlan = ref<TaskPlan | null>(null)
const priorityValue = ref<number>(5)

const ratingDialogTitle = computed(() =>
  ratingAction.value === 'start-plan' ? '开启间隔任务' : '完成间隔任务',
)

function openRatingDialog(action: 'start-plan' | 'complete-plan', target: TaskPlan) {
  ratingAction.value = action
  ratingTargetPlan.value = target
  ratingValue.value = 3
  showRatingDialog.value = true
}

async function submitRatingDialog(rating: number) {
  submittingRating.value = true
  try {
    if (ratingAction.value === 'start-plan' && ratingTargetPlan.value) {
      await sendMessage(`taskPlans/${ratingTargetPlan.value.ID}/start`, 'PATCH', {rating})
      ElMessage.success('已开启')
    } else if (ratingAction.value === 'complete-plan' && ratingTargetPlan.value) {
      await sendMessage(`taskPlans/${ratingTargetPlan.value.ID}/complete`, 'PATCH', {rating})
      ElMessage.success('已完成')
    }
    showRatingDialog.value = false
    ratingTargetPlan.value = null
    refreshAll()
    await refreshAllPlans()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  } finally {
    submittingRating.value = false
  }
}

function openPriorityDialog(plan: TaskPlan) {
  priorityTargetPlan.value = plan
  priorityValue.value = plan.Priority ?? 5
  showPriorityDialog.value = true
}

async function submitPriorityDialog() {
  if (!priorityTargetPlan.value) return
  if (priorityValue.value < 0) {
    ElMessage.error('优先级不能为负数')
    return
  }
  try {
    await sendMessage(`taskPlans/${priorityTargetPlan.value.ID}/priority`, 'PATCH', {
      priority: priorityValue.value,
    })
    showPriorityDialog.value = false
    priorityTargetPlan.value = null
    await refreshAllPlans()
    ElMessage.success('优先级已更新')
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

function openReview(plan: TaskPlan) {
  openCenterPanel('review', plan.ID, plan.Name)
}

const formData = ref({
  Name: '',
  PlanType: 'todo' as string,
  CronExpr: '',
  StartedAt: '',
  Remark: '',
  Link: '',
  ParentId: null as number | null,
  OrderNum: null as number | null,
  Priority: null as number | null,
})

const childrenMap = ref<Map<number, TaskPlanTree[]>>(new Map())
const tableRef = ref<any>(null)

function cloneTreeWithChildren(nodes: TaskPlanTree[]): TaskPlanTree[] {
  return nodes.map(node => ({
    ...node,
    Children: node.Children && node.Children.length > 0
        ? cloneTreeWithChildren(node.Children)
        : []
  }))
}

function processTreeData(nodes: TaskPlanTree[]): TaskPlanTree[] {
  fullTree.value = cloneTreeWithChildren(nodes)
  const map = new Map<number, TaskPlanTree[]>()

  function walk(list: TaskPlanTree[]): TaskPlanTree[] {
    return list.map(node => {
      if (node.Children && node.Children.length > 0) {
        map.set(node.ID, walk(node.Children))
        const {Children: _, ...rest} = node
        return {...rest, hasChildren: true} as TaskPlanTree
      }
      return node
    })
  }

  const result = walk(nodes)
  childrenMap.value = map
  return result
}

function collectDescendantIds(rootId: number): Set<number> {
  const result = new Set<number>()

  function walk(list: TaskPlanTree[]) {
    for (const node of list) {
      result.add(node.ID)
      const subChildren = childrenMap.value.get(node.ID) || []
      if (subChildren.length > 0) {
        walk(subChildren)
      }
    }
  }

  const children = childrenMap.value.get(rootId) || []
  walk(children)
  return result
}

function filterTree(nodes: TaskPlanTree[], excluded: Set<number>): TaskPlanTree[] {
  const result: TaskPlanTree[] = []
  for (const node of nodes) {
    if (excluded.has(node.ID)) continue
    const filteredChildren = node.Children && node.Children.length > 0
        ? filterTree(node.Children, excluded)
        : []
    result.push({...node, Children: filteredChildren})
  }
  return result
}

function loadChildren(row: TaskPlanTree, _treeNode: unknown, resolve: (data: TaskPlanTree[]) => void) {
  resolve(childrenMap.value.get(row.ID) || [])
}

// 是否为叶子节点：直接依据 childrenMap（后端完整树构建）判断，比 hasChildren 派生字段更可靠。
// 任何在 childrenMap 中有记录的节点都拥有子节点。
function isLeaf(row: TaskPlanTree): boolean {
  const kids = childrenMap.value.get(row.ID)
  return !kids || kids.length === 0
}

function captureExpandedIds(): Set<number> {
  if (!tableRef.value) return new Set()
  try {
    const treeData = tableRef.value.store?.states?.treeData
    if (!treeData) return new Set()
    const data = treeData.value ?? treeData
    const ids: number[] = []
    for (const [key, value] of Object.entries(data)) {
      if ((value as any).expanded) {
        const id = Number(key)
        if (!isNaN(id)) ids.push(id)
      }
    }
    console.log(`[tree] captureExpandedIds:`, ids)
    return new Set(ids)
  } catch (e) {
    console.error('[tree] captureExpandedIds error:', e)
    return new Set()
  }
}

async function restoreExpansion(idsToExpand: Set<number>) {
  if (idsToExpand.size === 0) return
  await nextTick()
  if (!tableRef.value) return
  await restoreExpansionRecursive(allPlans.value, idsToExpand)
}

async function restoreExpansionRecursive(rows: TaskPlanTree[], idsToExpand: Set<number>) {
  for (const row of rows) {
    if (idsToExpand.has(row.ID)) {
      tableRef.value.toggleRowExpansion(row, true)
      await nextTick()
      const children = childrenMap.value.get(row.ID) || []
      if (children.length > 0) {
        await restoreExpansionRecursive(children, idsToExpand)
      }
    }
  }
}

function resetLazyLoadedState(expandedIds: Set<number>) {
  if (!tableRef.value) return
  try {
    const treeData = tableRef.value.store?.states?.treeData
    if (!treeData) return
    for (const id of expandedIds) {
      if (treeData.value[id]) {
        treeData.value[id].loaded = false
      }
    }
  } catch (e) {
    console.error('[tree] resetLazyLoadedState error:', e)
  }
}

const selectableParents = computed(() => {
  if (!editingPlan.value) return fullTree.value
  const selfId = editingPlan.value.ID
  const excluded = collectDescendantIds(selfId)
  excluded.add(selfId)
  return filterTree(fullTree.value, excluded)
})

const statusMap: Record<string, { text: string, type: string }> = {
  created: {text: '已创建', type: 'info'},
  started: {text: '已开始', type: 'primary'},
  completed: {text: '已完成', type: 'success'},
  archived: {text: '已归档', type: 'info'},
}

function getProgress(row: TaskPlanTree): { completed: number; total: number; pct: string } | null {
  let completed = 0
  let total = 0

  function walk(list: TaskPlanTree[]) {
    for (const child of list) {
      const grandchildren = childrenMap.value.get(child.ID) || []
      if (grandchildren.length > 0) {
        // 非叶子节点：继续向下递归，不纳入统计
        walk(grandchildren)
      } else {
        // 叶子节点：已挂起的计划暂停统计，不计入进度
        if (child.IsSuspended) {
          continue
        }
        total++
        if (child.Status === 'started' || child.Status === 'completed' || child.Status === 'archived') {
          completed++
        }
      }
    }
  }

  const children = childrenMap.value.get(row.ID) || []
  if (children.length === 0) return null

  walk(children)

  if (total === 0) return null

  const pct = total > 0 ? Math.round((completed / total) * 100) + '%' : '0%'
  return {completed, total, pct}
}

// 任务计划的增删改与状态流转会影响待办任务列表（生成 / 完成 / 挂起等），
// 通过全局刷新信号通知侧边栏与待办任务页重新拉取。
function refreshAll() {
  refreshPendingTasks()
}

async function fetchAllPlans() {
  treeLoading.value = true
  error.value = ''
  const t0 = performance.now()

  const expandedIds = captureExpandedIds()
  const searchParam = props.searchText.trim()
  const isSearch = !!searchParam

  try {
    const result = await sendMessage('taskPlans/tree', 'GET', searchParam ? {search: searchParam} : undefined)
    if (Array.isArray(result)) {
      allPlans.value = processTreeData(result)
    }
    console.log(`[perf] fetchAllPlans api=${(performance.now() - t0).toFixed(0)}ms count=${result.length}`)
  } catch (e) {
    error.value = '获取任务计划失败'
    console.error(e)
  } finally {
    treeLoading.value = false
  }
  if (!error.value) {
    // 搜索结果较少时，直接展开所有树节点，方便查看
    if (isSearch && countAllNodes(allPlans.value) < 5) {
      await nextTick()
      await expandAllNodes()
    } else if (expandedIds.size > 0) {
      await nextTick()
      resetLazyLoadedState(expandedIds)
      await restoreExpansion(expandedIds)
    }
  }
}

async function refreshAllPlans() {
  await fetchAllPlans()
}

// 统计整棵树（含子节点）的节点总数，用于判断搜索结果条数。
function countAllNodes(nodes: TaskPlanTree[]): number {
  let count = 0
  for (const node of nodes) {
    count++
    const children = childrenMap.value.get(node.ID)
    if (children && children.length > 0) {
      count += countAllNodes(children)
    }
  }
  return count
}

// 展开所有树节点：收集顶层节点与 childrenMap 中记录的所有节点 ID，统一恢复展开。
async function expandAllNodes() {
  const ids = new Set<number>()
  for (const node of allPlans.value) ids.add(node.ID)
  for (const id of childrenMap.value.keys()) ids.add(id)
  if (ids.size === 0) return
  // 先重置懒加载状态：el-table 的 treeData 按 ID 缓存且不会随 data 替换而清空，
  // 若不将 loaded 置回 false，展开时不会重新触发 loadChildren，会残留上一次搜索的旧子树。
  await nextTick()
  resetLazyLoadedState(ids)
  await restoreExpansion(ids)
}

function resetForm() {
  formData.value = {
    Name: '',
    PlanType: 'todo',
    CronExpr: '',
    StartedAt: '',
    Remark: '',
    Link: '',
    ParentId: null,
    OrderNum: null,
    Priority: null,
  }
}

function openLink(ID: string) {
  sendMessage(`taskPlans/${ID}`, 'GET').then(res => {
    if (res.Link) {
      window.open(res.Link, '_blank')
    }
  })
}

async function createPlan() {
  if (!formData.value.Name.trim()) {
    ElMessage.error('请输入任务名称')
    return
  }

  if (formData.value.PlanType === 'cron' && !formData.value.CronExpr.trim()) {
    ElMessage.error('周期任务必须填写 cron 表达式')
    return
  }

  if (formData.value.PlanType === 'todo' && !formData.value.StartedAt.trim()) {
    ElMessage.error('待办任务必须填写开始时间')
    return
  }

  try {
    const payload: Record<string, any> = {
      Name: formData.value.Name.trim(),
      PlanType: formData.value.PlanType,
    }
    if (parentPlan.value) {
      payload.ParentId = parentPlan.value.ID
    }
    if (formData.value.CronExpr.trim()) {
      payload.CronExpr = formData.value.CronExpr.trim()
    }
    if (formData.value.StartedAt.trim()) {
      payload.StartedAt = formData.value.StartedAt.trim()
    }
    if (formData.value.Remark.trim()) {
      payload.Remark = formData.value.Remark.trim()
    }
    if (formData.value.Link.trim()) {
      payload.Link = formData.value.Link.trim()
    }
    if (formData.value.OrderNum !== null && formData.value.OrderNum !== undefined) {
      payload.OrderNum = formData.value.OrderNum
    }
    if (formData.value.Priority !== null && formData.value.Priority !== undefined) {
      payload.Priority = formData.value.Priority
    }

    await sendMessage('taskPlans/', 'POST', payload)
    showCreateDialog.value = false
    showAddChildDialog.value = false
    resetForm()
    parentPlan.value = null
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('创建成功')
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
    console.error(e)
  }
}

async function updatePlan() {
  if (!editingPlan.value) return
  if (!formData.value.Name.trim()) {
    ElMessage.error('请输入任务名称')
    return
  }

  try {
    await sendMessage(`taskPlans/${editingPlan.value.ID}`, 'PATCH', {
      Name: formData.value.Name.trim(),
      PlanType: formData.value.PlanType,
      CronExpr: formData.value.CronExpr.trim() || undefined,
      StartedAt: formData.value.StartedAt.trim() || undefined,
      Remark: formData.value.Remark.trim() || undefined,
      Link: formData.value.Link.trim() || undefined,
      ParentId: formData.value.ParentId || undefined,
      OrderNum: formData.value.OrderNum ?? undefined,
      Priority: formData.value.Priority ?? undefined,
    })
    showEditDialog.value = false
    editingPlan.value = null
    resetForm()
    await refreshAllPlans()
    ElMessage.success('修改成功')
  } catch (e: any) {
    ElMessage.error(e?.message || '修改失败')
    console.error(e)
  }
}

async function startPlan(plan: TaskPlan) {
  try {
    await ElMessageBox.confirm('确认开启此任务计划？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info',
    })
    await sendMessage(`taskPlans/${plan.ID}/start`, 'PATCH', {})
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已开启')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

async function completePlan(plan: TaskPlan) {
  if (plan.PlanType === 'interval') {
    openRatingDialog('complete-plan', plan)
    return
  }
  try {
    await ElMessageBox.confirm('确认完成此任务计划？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info',
    })
    await sendMessage(`taskPlans/${plan.ID}/complete`, 'PATCH', {})
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已完成')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

async function archivePlan(plan: TaskPlan) {
  try {
    await ElMessageBox.confirm('确认归档此任务计划？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await sendMessage(`taskPlans/${plan.ID}/archive`, 'PATCH', {})
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已归档')
  } catch (e) {
    if (e !== 'cancel') {
      console.error(e)
      ElMessage.error('归档失败')
    }
  }
}

async function deletePlan(plan: TaskPlan) {
  try {
    await ElMessageBox.confirm('确认删除此任务计划？删除后无法恢复。', '警告', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'error'
    })
    await sendMessage(`taskPlans/${plan.ID}`, 'DELETE')
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已删除')
  } catch (e) {
    if (e !== 'cancel') {
      console.error(e)
      ElMessage.error('删除失败')
    }
  }
}

async function suspendPlan(plan: TaskPlan) {
  try {
    await ElMessageBox.confirm(
        `确认挂起「${plan.Name}」？其下所有子任务都会一并挂起，待办列表中不再显示，恢复后可继续。`,
        '提示',
        {confirmButtonText: '挂起', cancelButtonText: '取消', type: 'warning'}
    )
    await sendMessage(`taskPlans/${plan.ID}/suspend`, 'PATCH', {})
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已挂起')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

async function resumePlan(plan: TaskPlan) {
  try {
    await ElMessageBox.confirm(
        `确认恢复「${plan.Name}」？其下所有被挂起的子任务都会一并恢复，重新出现在待办列表。`,
        '提示',
        {confirmButtonText: '恢复', cancelButtonText: '取消', type: 'info'}
    )
    await sendMessage(`taskPlans/${plan.ID}/resume`, 'PATCH', {})
    refreshAll()
    await refreshAllPlans()
    ElMessage.success('已恢复')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

async function openEditDialog(ID: string) {
  let res = await sendMessage(`taskPlans/${ID}`, 'GET')

  editingPlan.value = res
  formData.value = {
    Name: res.Name,
    PlanType: res.PlanType,
    CronExpr: res.CronExpr || '',
    StartedAt: res.StartedAt || '',
    Remark: res.Remark || '',
    Link: res.Link || '',
    ParentId: res.ParentID || null,
    OrderNum: res.OrderNum ?? null,
    Priority: res.Priority ?? null,
  }
  showEditDialog.value = true
}

function openAddChild(plan: TaskPlan) {
  parentPlan.value = plan
  resetForm()
  formData.value.PlanType = plan.PlanType
  showAddChildDialog.value = true
}

function openCreateRoot() {
  parentPlan.value = null
  resetForm()
  showCreateDialog.value = true
}

watch(() => props.searchText, () => {
  fetchAllPlans()
})

onMounted(async () => {
  const t0 = performance.now()
  fetchAllPlans()
  await nextTick()
  console.log(`[perf] onMounted → nextTick render=${(performance.now() - t0).toFixed(0)}ms`)
})

// 监听全局复习完成信号：居中面板完成复习后刷新任务计划，使复习次数等字段及时更新
watch(taskPlansVersion, () => {
  fetchAllPlans()
})
</script>

<template>
  <div>
    <div class="section-toolbar flex items-center justify-between">
      <span class="text-primary text-base section-title">任务计划</span>
      <div class="section-actions">
        <el-button size="small" type="primary" @click="openCreateRoot">
          + 新建任务
        </el-button>
      </div>
    </div>

    <el-alert
        v-if="error"
        type="error"
        :message="error"
        show-icon
        class="mb-sm"
        @close="error = ''"
    />

    <div v-if="allPlans.length === 0 && !treeLoading" class="empty-wrap">
      <el-empty description="暂无任务计划"/>
    </div>
    <el-table
        v-else
        ref="tableRef"
        :data="allPlans"
        row-key="ID"
        border
        stripe
        lazy
        :load="loadChildren"
        :tree-props="{ children: 'Children', hasChildren: 'hasChildren' }"
        v-loading="treeLoading"
        class="task-table"
    >
      <el-table-column label="名称" min-width="200">
        <template #default="{ row }">
          <span>{{ row.Name }}</span>
        </template>
      </el-table-column>
      <el-table-column label="ID" width="70">
        <template #default="{ row }">
          <span class="font-mono text-xs text-secondary">{{ row.ID }}</span>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="80">
        <template #default="{ row }">
          <el-tag size="small" :type="PLAN_TYPE_MAP[row.PlanType]?.type || 'info'">
            {{ PLAN_TYPE_MAP[row.PlanType]?.text || row.PlanType }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag v-if="row.IsSuspended" size="small" type="warning">已挂起</el-tag>
          <el-tag v-else size="small" :type="statusMap[row.Status]?.type || 'info'">
            {{ statusMap[row.Status]?.text || row.Status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="字数" width="80">
        <template #default="{ row }">
          <span v-if="row.ContentSize > 0">{{ row.ContentSize }}</span>
          <span v-else class="text-secondary">-</span>
        </template>
      </el-table-column>
      <el-table-column label="优先级" width="80">
        <template #default="{ row }">
          <span>{{ row.Priority ?? '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="进度" width="140">
        <template #default="{ row }">
          <template v-if="getProgress(row)">
            <span>{{ getProgress(row)!.pct }}({{ getProgress(row)!.completed }}/{{ getProgress(row)!.total }})</span>
          </template>
          <template v-else>
            <span class="text-secondary">-</span>
          </template>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <div class="op-actions">
            <el-button size="small" type="primary" text @click="openAddChild(row)">添加</el-button>
            <el-button v-if="row.Status === 'created' && isLeaf(row)" size="small" type="success" text @click="startPlan(row)">开启</el-button>
            <el-button v-if="row.HasLink" size="small" text @click="openLink(row.ID)">跳转</el-button>
            <el-dropdown trigger="click" style="margin-left: 0.25rem">
              <el-button size="small" text>更多</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="row.Status === 'created' && !isLeaf(row)" @click="startPlan(row)">开启</el-dropdown-item>
                  <el-dropdown-item @click="openEditDialog(row.ID)">修改</el-dropdown-item>
                  <el-dropdown-item @click="openPriorityDialog(row)">设置优先级</el-dropdown-item>
                  <el-dropdown-item v-if="!row.IsSuspended && row.Status !== 'archived' && row.Status !== 'completed'" @click="suspendPlan(row)">挂起
                  </el-dropdown-item>
                  <el-dropdown-item v-if="row.IsSuspended" @click="resumePlan(row)">恢复</el-dropdown-item>
                  <el-dropdown-item v-if="row.Status === 'started'" @click="completePlan(row)">完成</el-dropdown-item>
                  <el-dropdown-item v-if="row.Status === 'completed'" @click="archivePlan(row)">归档</el-dropdown-item>
                  <el-dropdown-item divided type="danger" @click="deletePlan(row)">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
        v-model="showCreateDialog"
        title="新建任务计划"
        width="31.25rem"
    >
      <TaskPlanForm :form-data="formData"/>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="createPlan">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showAddChildDialog"
        :title="`添加子任务 — ${parentPlan?.Name}`"
        width="31.25rem"
    >
      <TaskPlanForm :form-data="formData"/>
      <template #footer>
        <el-button @click="showAddChildDialog = false; parentPlan = null">取消</el-button>
        <el-button type="primary" @click="createPlan">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showEditDialog"
        title="修改任务计划"
        width="31.25rem"
    >
      <TaskPlanForm :form-data="formData" show-parent-select :selectable-parents="selectableParents"/>
      <template #footer>
        <el-button @click="showEditDialog = false; editingPlan = null">取消</el-button>
        <el-button type="primary" @click="updatePlan">保存修改</el-button>
      </template>
    </el-dialog>
    <RatingDialog
        v-model="showRatingDialog"
        v-model:rating="ratingValue"
        :title="ratingDialogTitle"
        :target-name="ratingTargetPlan?.Name || ''"
        :loading="submittingRating"
        @submit="submitRatingDialog"
    />

    <el-dialog
        v-model="showPriorityDialog"
        :title="`设置优先级 — ${priorityTargetPlan?.Name || ''}`"
        width="26.25rem"
    >
      <div class="postpone-content">
        <p class="text-secondary mb-sm">将递归应用到该计划及其所有子任务计划。</p>
        <el-input-number
            v-model="priorityValue"
            :min="0"
            :max="999"
            controls-position="right"
            style="width: 100%"
        />
      </div>
      <template #footer>
        <el-button @click="showPriorityDialog = false; priorityTargetPlan = null">取消</el-button>
        <el-button type="primary" @click="submitPriorityDialog">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.task-table {
  width: 100%;
}
</style>
