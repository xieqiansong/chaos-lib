<script setup lang="ts">
// 项目管理：左右两部分。
// 左：项目组（标准 CRUD，由 DataTable + projectGroupApi 驱动，无样板）。
// 右：项目（DataTable 配置驱动展示 + 分页 + 搜索；列表由后端合并「已认领 / 磁盘未认领」，
//      认领 / 移动 / 访问 / 复制路径 / 删除等动作经 #actions 插槽注入，保持基线纯净）。
import {onMounted, ref} from 'vue'
import {format} from 'date-fns'
import {ElMessage, ElMessageBox} from 'element-plus'
import DataTable from '@/components/DataTable.vue'
import DataFormDialog from '@/components/DataFormDialog.vue'
import type {DataTableApiParams, DataTableColumn, FormField} from '@/components/dataTable/types'
import {projectGroupApi, type ProjectGroup} from '@/api/projectGroup'
import {projectApi, type Project} from '@/api/project'
import {sendMessage} from '@/utils/api'

const selectedGroupId = ref<number | null>(null)
const projectsTable = ref<InstanceType<typeof DataTable> | null>(null)
const groupsForMove = ref<ProjectGroup[]>([])

// ── 左：项目组（标准 CRUD）──
const groupColumns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 120, searchable: true},
  {field: 'Remark', title: '备注', minWidth: 120},
  {field: '__actions', title: '操作', width: 140, type: 'actions', fixed: 'right'},
]
const groupFields: FormField[] = [
  {field: 'Name', title: '组名称', type: 'text', required: true, span: 24, placeholder: '项目组名称'},
  {field: 'OrderNum', title: '排序', type: 'number', span: 24, min: 0},
  {field: 'AbsolutePath', title: '根目录', type: 'text', required: true, span: 24, placeholder: '绝对路径，如 D:/code/mygroup'},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, span: 24, placeholder: '可选备注'},
]

// 点击左侧项目组 → 切换右侧过滤
function onGroupClick(row: ProjectGroup) {
  selectedGroupId.value = row.ID
  projectsTable.value?.refresh()
}

// ── 右：项目（自定义取数 + 动作插槽）──
const projectColumns: DataTableColumn[] = [
  {field: 'Name', title: '名称', minWidth: 140, searchable: true},
  {field: 'AbsolutePath', title: '绝对路径', minWidth: 140},
  {field: 'Remark', title: '备注', minWidth: 140},
  {field: 'LastAccessedAt', title: '上次访问', width: 160, type: 'datetime'},
  {field: 'Claimed', title: '状态', width: 90, formatter: (row: any) => (row.Claimed ? '已认领' : '未认领')},
  {field: '__actions', title: '操作', width: 180, type: 'actions', fixed: 'right'},
]
// 编辑弹窗字段（仅允许改名称 / Git / 备注，路径经移动流程改写）
const projectEditFields: FormField[] = [
  {field: 'Name', title: '名称', type: 'text', required: true, span: 24, placeholder: '项目名称'},
  {field: 'GitURL', title: 'Git 地址', type: 'text', span: 24, placeholder: '可选 Git 仓库地址'},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, span: 24, placeholder: '可选备注'},
]
// 新建弹窗字段（需指定所属组下的路径）
const projectCreateFields: FormField[] = [
  {field: 'Name', title: '名称', type: 'text', span: 12, placeholder: '留空取目录名'},
  {field: 'AbsolutePath', title: '绝对路径', type: 'text', span: 12, placeholder: '与相对路径二选一'},
  {field: 'RelativePath', title: '相对路径', type: 'text', span: 12, placeholder: '相对组根目录，如 proj'},
  {field: 'GitURL', title: 'Git 地址', type: 'text', span: 12, placeholder: '可选'},
  {field: 'Remark', title: '备注', type: 'textarea', rows: 2, span: 24, placeholder: '可选备注'},
]

// 自定义取数：仅返回所选组的项目（合并已认领 + 未认领）；未选组则返回空。
async function projectsFetch(params: DataTableApiParams) {
  if (selectedGroupId.value == null) return {rows: [], total: 0}
  const query: Record<string, any> = {
    page: params.page,
    size: params.pageSize,
    groupId: selectedGroupId.value,
  }
  for (const [k, v] of Object.entries(params.search ?? {})) {
    if (v !== '' && v != null) query[toSnake(k)] = v
  }
  const res = await sendMessage('projects', 'GET', query)
  const rows = (res?.items ?? res?.rows ?? []) as any[]
  const total = res?.total ?? rows.length
  return {rows, total}
}

// ── 弹窗状态 ──
const showCreate = ref(false)
const showEdit = ref(false)
const showDetail = ref(false)
const showMove = ref(false)
const saving = ref(false)
const form = ref<Record<string, any>>({})
const editId = ref(0)
const detailItem = ref<Project | null>(null)
const moveForm = ref({TargetGroupID: 0, TargetRelativePath: ''})
const moveId = ref(0)

function defaultForm(fields: FormField[]): Record<string, any> {
  const f: Record<string, any> = {}
  for (const field of fields) {
    if (field.type === 'number') f[field.field] = field.defaultValue ?? 0
    else if (field.type === 'switch') f[field.field] = field.defaultValue ?? false
    else f[field.field] = field.defaultValue ?? ''
  }
  return f
}

function openCreateProject() {
  if (selectedGroupId.value == null) {
    ElMessage.warning('请先选择左侧项目组')
    return
  }
  form.value = defaultForm(projectCreateFields)
  showCreate.value = true
}

function openEditProject(row: Project) {
  editId.value = row.ID
  form.value = {Name: row.Name, GitURL: row.GitURL ?? '', Remark: row.Remark ?? ''}
  showEdit.value = true
}

function openDetail(row: Project) {
  detailItem.value = row
  showDetail.value = true
}

function openMove(row: Project) {
  moveId.value = row.ID
  moveForm.value = {TargetGroupID: row.GroupID, TargetRelativePath: row.RelativePath}
  showMove.value = true
}

async function saveCreate() {
  saving.value = true
  try {
    await projectApi.create({GroupID: selectedGroupId.value!, ...form.value})
    ElMessage.success('创建成功')
    showCreate.value = false
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  } finally {
    saving.value = false
  }
}

async function saveEdit() {
  saving.value = true
  try {
    await projectApi.update(editId.value, {
      Name: form.value.Name,
      GitURL: form.value.GitURL || null,
      Remark: form.value.Remark || null,
    })
    ElMessage.success('更新成功')
    showEdit.value = false
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '更新失败')
  } finally {
    saving.value = false
  }
}

async function doMove() {
  try {
    await projectApi.move(moveId.value, moveForm.value.TargetGroupID, moveForm.value.TargetRelativePath || undefined)
    ElMessage.success('移动成功')
    showMove.value = false
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '移动失败')
  }
}

async function claimProject(row: Project) {
  try {
    await projectApi.claim({
      GroupID: row.GroupID,
      Name: row.Name,
      AbsolutePath: row.AbsolutePath,
      RelativePath: row.RelativePath,
    })
    ElMessage.success('认领成功')
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '认领失败')
  }
}

async function accessProject(row: Project) {
  try {
    await projectApi.access(row.ID)
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '访问失败')
  }
}

async function deleteProject(row: Project) {
  try {
    await ElMessageBox.confirm(
      `将永久删除项目「${row.Name}」及其磁盘目录，此操作不可恢复，确定继续？`,
      '删除项目',
      {confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning', confirmButtonClass: 'el-button--danger'},
    )
  } catch {
    return
  }
  try {
    await projectApi.remove(row.ID)
    ElMessage.success('删除成功')
    projectsTable.value?.refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

// 复制项目绝对路径到剪贴板（后端以服务运行，无桌面会话，无法直接打开资源管理器）
async function copyPath(p: Project) {
  if (!p.AbsolutePath) return
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(p.AbsolutePath)
    } else {
      const ta = document.createElement('textarea')
      ta.value = p.AbsolutePath
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    ElMessage.success('已复制路径：' + p.AbsolutePath)
  } catch (e: any) {
    ElMessage.error('复制失败，请手动复制：' + p.AbsolutePath)
  }
}

function formatTime(value: string | null | undefined): string {
  if (!value) return '—'
  const d = new Date(value)
  if (isNaN(d.getTime())) return '—'
  return format(d, 'yyyy-MM-dd HH:mm:ss')
}

// 驼峰字段名 → snake_case，用于把前端列字段名翻译成后端查询参数
function toSnake(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
}

onMounted(async () => {
  // 拉取组列表（供移动选择 + 默认选中第一个组）
  try {
    const res = await projectGroupApi.fetch({page: 1, pageSize: 200, search: {}})
    groupsForMove.value = res.rows
    if (res.rows.length) {
      selectedGroupId.value = res.rows[0].ID
      projectsTable.value?.refresh()
    }
  } catch (e) {
    console.error(e)
  }
})
</script>

<template>
  <div>
    <el-row :gutter="16">
      <!-- 左：项目组 -->
      <el-col :span="7">
        <DataTable
            :api="projectGroupApi"
            :columns="groupColumns"
            :fields="groupFields"
            title="项目组"
            :default-sort="{field: 'OrderNum', order: 'ascending'}"
            @row-click="onGroupClick"
        />
      </el-col>

      <!-- 右：项目 -->
      <el-col :span="17">
        <DataTable
            ref="projectsTable"
            :api="projectsFetch"
            :columns="projectColumns"
            title="项目"
        >
          <template #toolbar>
            <el-button size="small" type="primary" :disabled="selectedGroupId == null" @click="openCreateProject">
              + 新建项目
            </el-button>
          </template>
          <template #actions="{row}">
            <div class="op-actions">
              <template v-if="row.Claimed">
                <el-button size="small" text @click="openDetail(row)">详情</el-button>
                <el-button size="small" text type="primary" @click="openEditProject(row)">编辑</el-button>
                <el-button size="small" text type="danger" @click="deleteProject(row)">删除</el-button>
                <el-button size="small" text @click="openMove(row)">移动</el-button>
                <el-button size="small" text @click="accessProject(row)">访问</el-button>
                <el-button size="small" text @click="copyPath(row)">复制路径</el-button>
              </template>
              <el-button v-else size="small" type="success" text @click="claimProject(row)">认领</el-button>
            </div>
          </template>
        </DataTable>
      </el-col>
    </el-row>

    <!-- 新建项目 -->
    <DataFormDialog
        v-model="showCreate"
        title="新建项目"
        mode="create"
        :fields="projectCreateFields"
        :form="form"
        :saving="saving"
        @save="saveCreate"
    />

    <!-- 编辑项目 -->
    <DataFormDialog
        v-model="showEdit"
        title="编辑项目"
        mode="edit"
        :fields="projectEditFields"
        :form="form"
        :saving="saving"
        @save="saveEdit"
    />

    <!-- 详情 -->
    <el-dialog v-model="showDetail" title="项目详情" width="40rem">
      <div v-if="detailItem" class="detail-body">
        <div class="detail-row">
          <span class="detail-label">状态</span>
          <el-tag :type="detailItem.Claimed ? 'success' : 'warning'" size="small">
            {{ detailItem.Claimed ? '已认领' : '未认领' }}
          </el-tag>
        </div>
        <div class="detail-row">
          <span class="detail-label">名称</span>
          <span class="text-primary">{{ detailItem.Name }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">绝对路径</span>
          <span class="text-xs font-mono truncate">{{ detailItem.AbsolutePath }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">相对路径</span>
          <span class="text-xs font-mono truncate">{{ detailItem.RelativePath || '—' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">Git 地址</span>
          <span class="text-xs truncate">{{ detailItem.GitURL || '—' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">备注</span>
          <span class="text-xs">{{ detailItem.Remark || '—' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">添加时间</span>
          <span class="text-xs">{{ formatTime(detailItem.CreatedAt) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">上次访问</span>
          <span class="text-xs">{{ formatTime(detailItem.LastAccessedAt) }}</span>
        </div>
      </div>
      <template #footer>
        <template v-if="detailItem?.Claimed">
          <el-button size="small" @click="accessProject(detailItem); showDetail = false">访问</el-button>
          <el-button size="small" @click="copyPath(detailItem); showDetail = false">复制路径</el-button>
          <el-button size="small" @click="openMove(detailItem); showDetail = false">移动</el-button>
          <el-button size="small" @click="openEditProject(detailItem); showDetail = false">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteProject(detailItem); showDetail = false">删除</el-button>
        </template>
        <el-button v-else type="success" size="small" @click="claimProject(detailItem); showDetail = false">认领</el-button>
      </template>
    </el-dialog>

    <!-- 移动 -->
    <el-dialog v-model="showMove" title="移动项目" width="37.5rem">
      <el-form :model="moveForm" label-width="6.25rem">
        <el-form-item label="目标项目组">
          <el-select v-model="moveForm.TargetGroupID" placeholder="选择目标项目组" style="width: 100%">
            <el-option v-for="g in groupsForMove" :key="g.ID" :label="g.Name" :value="g.ID"/>
          </el-select>
        </el-form-item>
        <el-form-item label="目标相对路径">
          <el-input v-model="moveForm.TargetRelativePath" placeholder="相对目标组根目录的路径，如 proj 或 sub/proj"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showMove = false">取消</el-button>
        <el-button type="primary" @click="doMove">移动</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.detail-body {
  max-height: 60vh;
  overflow-y: auto;
}

.detail-row {
  display: flex;
  align-items: flex-start;
  gap: var(--space-md);
  padding: var(--space-sm) 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.detail-label {
  width: 80px;
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-size: var(--el-font-size-small);
}
</style>
