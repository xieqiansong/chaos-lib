<script setup lang="ts">
import {computed, nextTick, onMounted, ref, watch} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage, ElMessageBox} from 'element-plus'
import PendingTasks from '../components/PendingTasks.vue'
import {refreshPendingTasks} from '@/utils/pendingTasksStore'

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
const loading = ref(false)
const activeTab = ref<'pending' | 'all'>('all')

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showAddChildDialog = ref(false)
const editingPlan = ref<TaskPlan | null>(null)
const parentPlan = ref<TaskPlan | null>(null)

const showRatingDialog = ref(false)
const ratingAction = ref<'start-plan' | 'complete-plan'>('complete-plan')
const ratingTargetPlan = ref<TaskPlan | null>(null)
const ratingValue = ref<number | null>(3)

const showPriorityDialog = ref(false)
const priorityTargetPlan = ref<TaskPlan | null>(null)
const priorityValue = ref<number>(5)

const pendingRef = ref<InstanceType<typeof PendingTasks> | null>(null)

const ratingOptions = [
  {value: 1, label: 'Again（忘记了）', type: 'danger'},
  {value: 2, label: 'Hard（记得但困难）', type: 'warning'},
  {value: 3, label: 'Good（正常记得）', type: 'primary'},
  {value: 4, label: 'Easy（太简单）', type: 'success'},
]

const ratingDialogTitle = computed(() => {
  if (ratingAction.value === 'start-plan') {
    return `开启间隔任务 — ${ratingTargetPlan.value?.Name || ''}`
  }
  return `完成间隔任务 — ${ratingTargetPlan.value?.Name || ''}`
})

function openRatingDialog(action: 'start-plan' | 'complete-plan', target: TaskPlan) {
  ratingAction.value = action
  ratingTargetPlan.value = target
  ratingValue.value = 3
  showRatingDialog.value = true
}

async function submitRatingDialog() {
  if (ratingValue.value === null) {
    ElMessage.error('请选择评分')
    return
  }
  try {
    if (ratingAction.value === 'start-plan' && ratingTargetPlan.value) {
      await sendMessage(`taskPlans/${ratingTargetPlan.value.ID}/start`, 'PATCH', {
        rating: ratingValue.value,
      })
      ElMessage.success('已开启')
    } else if (ratingAction.value === 'complete-plan' && ratingTargetPlan.value) {
      await sendMessage(`taskPlans/${ratingTargetPlan.value.ID}/complete`, 'PATCH', {
        rating: ratingValue.value,
      })
      ElMessage.success('已完成')
    }
    showRatingDialog.value = false
    ratingTargetPlan.value = null
    await refreshAll()
    await refreshAllPlans()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
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

const planTypeMap: Record<string, { text: string, type: string }> = {
  todo: {text: '待办', type: 'primary'},
  cron: {text: '周期', type: 'success'},
  interval: {text: '间隔', type: 'warning'},
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
        // 叶子节点：纳入统计
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
  return { completed, total, pct }
}

async function refreshAll() {
  loading.value = true
  refreshPendingTasks()
  await pendingRef.value?.loadPendingTasks()
  loading.value = false
}

async function fetchAllPlans() {
  treeLoading.value = true
  error.value = ''
  const t0 = performance.now()

  const expandedIds = captureExpandedIds()

  try {
    const searchParam = props.searchText.trim()
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
  if (!error.value && expandedIds.size > 0) {
    await nextTick()
    resetLazyLoadedState(expandedIds)
    await restoreExpansion(expandedIds)
  }
}

async function refreshAllPlans() {
  if (activeTab.value === 'all') {
    await fetchAllPlans()
  }
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

function openLink(link: string) {
  window.open(link, '_blank')
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
    await refreshAll()
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
    await refreshAll()
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
    await refreshAll()
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
    await refreshAll()
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
    await refreshAll()
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
    await refreshAll()
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
    await refreshAll()
    await refreshAllPlans()
    ElMessage.success('已恢复')
  } catch (e: any) {
    if (e === 'cancel') return
    ElMessage.error(e?.message || '操作失败')
    console.error(e)
  }
}

function openEditDialog(plan: TaskPlan) {
  editingPlan.value = plan
  formData.value = {
    Name: plan.Name,
    PlanType: plan.PlanType,
    CronExpr: plan.CronExpr || '',
    StartedAt: plan.StartedAt || '',
    Remark: plan.Remark || '',
    Link: plan.Link || '',
    ParentId: plan.ParentID || null,
    OrderNum: plan.OrderNum ?? null,
    Priority: plan.Priority ?? null,
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
  refreshAll()
  if (activeTab.value === 'all') {
    fetchAllPlans()
  }
})

watch(activeTab, async (tab) => {
  if (tab === 'all') {
    const t0 = performance.now()
    await fetchAllPlans()
    await nextTick()
    console.log(`[perf] switch to all tab total=${(performance.now() - t0).toFixed(0)}ms`)
  }
})

onMounted(async () => {
  const t0 = performance.now()
  refreshAll()
  fetchAllPlans()
  await nextTick()
  console.log(`[perf] onMounted → nextTick render=${(performance.now() - t0).toFixed(0)}ms`)
})
</script>

<template>
  <div>
    <div class="section-toolbar flex items-center justify-between">
      <span class="text-primary text-base section-title">任务管理</span>
      <el-tabs v-model="activeTab" class="task-tabs flex-1 ml-md">
        <el-tab-pane label="任务计划" name="all"/>
        <el-tab-pane label="待办任务" name="pending"/>
      </el-tabs>
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

    <el-skeleton v-if="loading && activeTab === 'all'" :rows="5" animated/>

    <template v-if="activeTab === 'pending'">
      <PendingTasks ref="pendingRef" view="table" @refresh="refreshAllPlans"/>
    </template>

    <template v-if="activeTab === 'all'">
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
            <el-tag size="small" :type="planTypeMap[row.PlanType]?.type || 'info'">
              {{ planTypeMap[row.PlanType]?.text || row.PlanType }}
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
              <el-button v-if="row.Link" size="small" text @click="openLink(row.Link!)">跳转</el-button>
              <el-dropdown trigger="click" style="margin-left: 4px">
                <el-button size="small" text>更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="row.Status === 'created' && !isLeaf(row)" @click="startPlan(row)">开启</el-dropdown-item>
                    <el-dropdown-item @click="openEditDialog(row)">修改</el-dropdown-item>
                    <el-dropdown-item @click="openPriorityDialog(row)">设置优先级</el-dropdown-item>
                    <el-dropdown-item v-if="!row.IsSuspended && row.Status !== 'archived' && row.Status !== 'completed'" @click="suspendPlan(row)">挂起</el-dropdown-item>
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
    </template>

    <el-dialog
        v-model="showCreateDialog"
        title="新建任务计划"
        width="500px"
    >
      <el-form label-position="top">
        <el-form-item label="任务名称">
          <el-input v-model="formData.Name" placeholder="请输入任务名称"/>
        </el-form-item>
        <el-form-item label="任务类型">
          <el-select v-model="formData.PlanType">
            <el-option label="待办任务" value="todo"/>
            <el-option label="周期重复任务" value="cron"/>
            <el-option label="间隔任务" value="interval"/>
          </el-select>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'cron'" label="Cron 表达式">
          <el-input v-model="formData.CronExpr" placeholder="如: 0 8 * * * (每天8:00)"/>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'todo'" label="开始时间">
          <el-date-picker
              v-model="formData.StartedAt"
              type="datetime"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD[T]HH:mm:ssZ"
              placeholder="选择开始时间"
              style="width: 100%"
          />
        </el-form-item>
        <div class="form-row">
          <el-form-item label="优先级">
            <el-input-number v-model="formData.Priority" :min="0" :controls="false" placeholder="数值越大越优先" style="width: 100%"/>
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="formData.OrderNum" :min="0" :controls="false" placeholder="控制显示顺序" style="width: 100%"/>
          </el-form-item>
        </div>
        <el-form-item label="关联链接">
          <el-input v-model="formData.Link" placeholder="可选"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="createPlan">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showAddChildDialog"
        :title="`添加子任务 — ${parentPlan?.Name}`"
        width="500px"
    >
      <el-form label-position="top">
        <el-form-item label="任务名称">
          <el-input v-model="formData.Name" placeholder="请输入子任务名称"/>
        </el-form-item>
        <el-form-item label="任务类型">
          <el-select v-model="formData.PlanType">
            <el-option label="待办任务" value="todo"/>
            <el-option label="周期重复任务" value="cron"/>
            <el-option label="间隔任务" value="interval"/>
          </el-select>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'cron'" label="Cron 表达式">
          <el-input v-model="formData.CronExpr" placeholder="如: 0 8 * * * (每天8:00)"/>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'todo'" label="开始时间">
          <el-date-picker
              v-model="formData.StartedAt"
              type="datetime"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD[T]HH:mm:ssZ"
              placeholder="选择开始时间"
              style="width: 100%"
          />
        </el-form-item>
        <div class="form-row">
          <el-form-item label="优先级">
            <el-input-number v-model="formData.Priority" :min="0" :controls="false" placeholder="数值越大越优先" style="width: 100%"/>
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="formData.OrderNum" :min="0" :controls="false" placeholder="控制显示顺序" style="width: 100%"/>
          </el-form-item>
        </div>
        <el-form-item label="关联链接">
          <el-input v-model="formData.Link" placeholder="可选"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddChildDialog = false; parentPlan = null">取消</el-button>
        <el-button type="primary" @click="createPlan">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showEditDialog"
        title="修改任务计划"
        width="500px"
    >
      <el-form label-position="top">
        <el-form-item label="父任务">
          <el-tree-select
              v-model="formData.ParentId"
              :data="selectableParents"
              :props="{ label: 'Name', value: 'ID', children: 'Children' }"
              node-key="ID"
              check-strictly
              clearable
              placeholder="根节点（无父任务）"
              style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="任务名称">
          <el-input v-model="formData.Name" placeholder="请输入任务名称"/>
        </el-form-item>
        <el-form-item label="任务类型">
          <el-select v-model="formData.PlanType">
            <el-option label="待办任务" value="todo"/>
            <el-option label="周期重复任务" value="cron"/>
            <el-option label="间隔任务" value="interval"/>
          </el-select>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'cron'" label="Cron 表达式">
          <el-input v-model="formData.CronExpr" placeholder="如: 0 8 * * * (每天8:00)"/>
        </el-form-item>
        <el-form-item v-if="formData.PlanType === 'todo'" label="开始时间">
          <el-date-picker
              v-model="formData.StartedAt"
              type="datetime"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD[T]HH:mm:ssZ"
              placeholder="选择开始时间"
              style="width: 100%"
          />
        </el-form-item>
        <div class="form-row">
          <el-form-item label="优先级">
            <el-input-number v-model="formData.Priority" :min="0" :controls="false" placeholder="数值越大越优先" style="width: 100%"/>
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="formData.OrderNum" :min="0" :controls="false" placeholder="控制显示顺序" style="width: 100%"/>
          </el-form-item>
        </div>
        <el-form-item label="关联链接">
          <el-input v-model="formData.Link" placeholder="可选"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="formData.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false; editingPlan = null">取消</el-button>
        <el-button type="primary" @click="updatePlan">保存修改</el-button>
      </template>
    </el-dialog>
    <el-dialog
        v-model="showRatingDialog"
        :title="ratingDialogTitle"
        width="480px"
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
        <el-button @click="showRatingDialog = false; ratingTargetPlan = null">取消</el-button>
        <el-button type="primary" @click="submitRatingDialog">确认</el-button>
      </template>
    </el-dialog>

    <el-dialog
        v-model="showPriorityDialog"
        :title="`设置优先级 — ${priorityTargetPlan?.Name || ''}`"
        width="420px"
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
.task-tabs {
  margin-bottom: 0;
}

.task-table {
  width: 100%;
}

.form-row {
  display: flex;
  gap: var(--space-md);
}

.form-row .el-form-item {
  flex: 1;
  margin-bottom: 18px;
}

.expand-icon {
  cursor: pointer;
  user-select: none;
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
  width: 1rem;
  display: inline-block;
  text-align: center;
  flex-shrink: 0;
}

.expand-icon-placeholder {
  width: 1rem;
  flex-shrink: 0;
}
</style>