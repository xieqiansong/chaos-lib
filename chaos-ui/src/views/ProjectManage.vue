<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {format} from 'date-fns'
import {sendMessage} from '@/utils/api'

interface ProjectGroup {
  ID: number
  Name: string
  OrderNum: number
  AbsolutePath: string
  Remark: string | null
  CreatedAt: string
  UpdatedAt: string
}

interface Project {
  ID: number
  GroupID: number
  Name: string
  AbsolutePath: string
  RelativePath: string
  GitURL: string | null
  Remark: string | null
  LastAccessedAt: string | null
  CreatedAt: string
  Claimed?: boolean
}

const groups = ref<ProjectGroup[]>([])
const projects = ref<Project[]>([])
const selectedGroupId = ref<number | null>(null)
const loading = ref(false)
const error = ref('')

// 项目组弹窗
const showGroupModal = ref(false)
const isGroupEdit = ref(false)
const groupForm = ref({ID: 0, Name: '', OrderNum: 0, AbsolutePath: '', Remark: ''})

// 项目弹窗
const showProjectModal = ref(false)
const isProjectEdit = ref(false)
const projectForm = ref({
  ID: 0, GroupID: 0, Name: '', AbsolutePath: '', RelativePath: '', GitURL: '', Remark: ''
})

// 移动弹窗
const showMoveModal = ref(false)
const moving = ref(false)
const moveProjectId = ref(0)
const moveForm = ref({TargetGroupID: 0, TargetRelativePath: ''})

// 详情弹窗
const showDetail = ref(false)
const detailItem = ref<Project | null>(null)

function openDetail(p: Project) {
  detailItem.value = p
  showDetail.value = true
}

// formatTime 将后端返回的 ISO 时间字符串格式化为本地可读时间；空值返回占位符。
function formatTime(value: string | null | undefined): string {
  if (!value) return '—'
  const d = new Date(value)
  if (isNaN(d.getTime())) return '—'
  return format(d, 'yyyy-MM-dd HH:mm:ss')
}

const selectedGroup = computed(() =>
  groups.value.find(g => g.ID === selectedGroupId.value) || null
)

// 项目列表请求令牌：每次重新拉取自增，用于丢弃过期的旧响应
let projectReqToken = 0

async function fetchGroups() {
  loading.value = true
  error.value = ''
  try {
    groups.value = await sendMessage('projectGroups', 'GET')
    if (selectedGroupId.value === null && groups.value.length > 0) {
      selectedGroupId.value = groups.value[0].ID
    }
    if (selectedGroupId.value !== null) {
      await fetchProjects()
    } else {
      projects.value = []
    }
  } catch (e) {
    error.value = '获取项目组失败'
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function fetchProjects() {
  if (selectedGroupId.value === null) {
    projects.value = []
    return
  }
  // 请求令牌：仅应用最新一次请求的结果，避免快速切换分组时旧响应覆盖新数据
  const token = ++projectReqToken
  error.value = ''
  try {
    const data = await sendMessage('projects', 'GET', {groupId: selectedGroupId.value})
    if (token !== projectReqToken) return
    projects.value = data
  } catch (e) {
    if (token !== projectReqToken) return
    error.value = '获取项目失败'
    console.error(e)
  }
}

function selectGroup(id: number) {
  selectedGroupId.value = id
  projects.value = [] // 立即清空，确保右侧列表在请求返回前就有变化
  fetchProjects()
}

// ===== 项目组 =====
function openCreateGroup() {
  isGroupEdit.value = false
  groupForm.value = {ID: 0, Name: '', OrderNum: 0, AbsolutePath: '', Remark: ''}
  showGroupModal.value = true
}

function openEditGroup(g: ProjectGroup) {
  isGroupEdit.value = true
  groupForm.value = {ID: g.ID, Name: g.Name, OrderNum: g.OrderNum, AbsolutePath: g.AbsolutePath, Remark: g.Remark || ''}
  showGroupModal.value = true
}

async function saveGroup() {
  if (!groupForm.value.Name.trim()) {
    error.value = '请填写组名称'
    return
  }
  if (!groupForm.value.AbsolutePath.trim()) {
    error.value = '请填写根目录绝对路径'
    return
  }
  try {
    if (isGroupEdit.value) {
      await sendMessage(`projectGroups/${groupForm.value.ID}`, 'PATCH', {
        Name: groupForm.value.Name,
        OrderNum: groupForm.value.OrderNum,
        AbsolutePath: groupForm.value.AbsolutePath,
        Remark: groupForm.value.Remark || null
      })
    } else {
      await sendMessage('projectGroups', 'POST', {
        Name: groupForm.value.Name,
        OrderNum: groupForm.value.OrderNum,
        AbsolutePath: groupForm.value.AbsolutePath,
        Remark: groupForm.value.Remark || null
      })
    }
    showGroupModal.value = false
    await fetchGroups()
  } catch (e) {
    error.value = '保存项目组失败'
    console.error(e)
  }
}

async function deleteGroup(g: ProjectGroup) {
  if (!confirm(`确认删除项目组「${g.Name}」？其下项目需先移除，或用 cascade 级联删除。`)) return
  try {
    await sendMessage(`projectGroups/${g.ID}`, 'DELETE')
    if (selectedGroupId.value === g.ID) selectedGroupId.value = null
    await fetchGroups()
  } catch (e) {
    error.value = '删除失败（含项目时需先清空，或后端级联）'
    console.error(e)
  }
}

// ===== 项目 =====
function openCreateProject() {
  if (selectedGroupId.value === null) {
    error.value = '请先选择一个项目组'
    return
  }
  isProjectEdit.value = false
  projectForm.value = {
    ID: 0, GroupID: selectedGroupId.value, Name: '', AbsolutePath: '', RelativePath: '', GitURL: '', Remark: ''
  }
  showProjectModal.value = true
}

function openEditProject(p: Project) {
  isProjectEdit.value = true
  projectForm.value = {
    ID: p.ID, GroupID: p.GroupID, Name: p.Name,
    AbsolutePath: p.AbsolutePath, RelativePath: p.RelativePath,
    GitURL: p.GitURL || '', Remark: p.Remark || ''
  }
  showProjectModal.value = true
}

async function saveProject() {
  const f = projectForm.value
  if (!f.Name.trim()) {
    error.value = '请填写项目名称'
    return
  }
  if (!f.AbsolutePath.trim() && !f.RelativePath.trim()) {
    error.value = '请填写绝对路径或相对路径（二选一）'
    return
  }
  try {
    if (isProjectEdit.value) {
      await sendMessage(`projects/${f.ID}`, 'PATCH', {
        Name: f.Name,
        GitURL: f.GitURL || null,
        Remark: f.Remark || null
      })
    } else {
      await sendMessage('projects', 'POST', {
        GroupID: f.GroupID,
        Name: f.Name,
        AbsolutePath: f.AbsolutePath || undefined,
        RelativePath: f.RelativePath || undefined,
        GitURL: f.GitURL || null,
        Remark: f.Remark || null
      })
    }
    showProjectModal.value = false
    await fetchProjects()
  } catch (e) {
    error.value = '保存项目失败'
    console.error(e)
  }
}

// ===== 移动 =====
function openMove(p: Project) {
  moveProjectId.value = p.ID
  moveForm.value = {TargetGroupID: p.GroupID, TargetRelativePath: p.RelativePath}
  showMoveModal.value = true
}

async function doMove() {
  if (moveForm.value.TargetGroupID === 0) {
    error.value = '请选择目标项目组'
    return
  }
  moving.value = true
  error.value = ''
  try {
    const res = await sendMessage(`projects/${moveProjectId.value}/move`, 'PATCH', {
      TargetGroupID: moveForm.value.TargetGroupID,
      TargetRelativePath: moveForm.value.TargetRelativePath || undefined
    })
    if (res.recycleWarning) {
      error.value = '移动完成，但原目录未送入回收站: ' + res.recycleWarning
    }
    showMoveModal.value = false
    await fetchProjects()
  } catch (e) {
    error.value = '移动项目失败'
    console.error(e)
  } finally {
    moving.value = false
  }
}

// ===== 认领（未入库的磁盘子目录）=====
async function claimProject(item: Project) {
  try {
    await sendMessage('projects', 'POST', {
      GroupID: item.GroupID,
      Name: item.Name,
      AbsolutePath: item.AbsolutePath,
      RelativePath: item.RelativePath,
      GitURL: null,
      Remark: null
    })
    await fetchProjects()
  } catch (e) {
    error.value = '认领失败'
    console.error(e)
  }
}

// ===== 访问 / 删除 =====
async function accessProject(p: Project) {
  try {
    await sendMessage(`projects/${p.ID}/access`, 'PATCH')
    await fetchProjects()
  } catch (e) {
    console.error(e)
  }
}

async function deleteProject(p: Project) {
  if (!confirm(`确认删除项目「${p.Name}」？`)) return
  try {
    await sendMessage(`projects/${p.ID}`, 'DELETE')
    await fetchProjects()
  } catch (e) {
    error.value = '删除失败'
    console.error(e)
  }
}

onMounted(fetchGroups)
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">本地项目管理</span>
      <span class="text-secondary text-xs">项目组 / 项目文件夹（可移动、备注、记录访问）</span>
    </div>

    <el-alert
        v-if="error"
        type="error"
        :message="error"
        show-icon
        class="mb-sm"
        @close="error = ''"
    />

    <el-row :gutter="16">
      <!-- 左：项目组 -->
      <el-col :span="6">
        <div class="panel-header">
          <span class="text-primary text-sm" style="font-weight:600">项目组</span>
          <el-button size="small" type="primary" @click="openCreateGroup">+ 新建组</el-button>
        </div>
        <el-skeleton v-if="loading" :rows="4" animated/>
        <div v-else-if="groups.length === 0" class="empty-wrap">
          <el-empty description="暂无项目组"/>
        </div>
        <ul v-else class="group-list">
          <li
              v-for="g in groups"
              :key="g.ID"
              class="group-item"
              :class="{active: g.ID === selectedGroupId}"
              @click="selectGroup(g.ID)">
            <div class="group-item-main">
              <div class="text-primary text-sm truncate">{{ g.Name }}</div>
              <div class="text-placeholder text-xs font-mono truncate">{{ g.AbsolutePath }}</div>
            </div>
            <div class="group-item-ops" @click.stop>
              <el-button size="small" text @click="openEditGroup(g)">编辑</el-button>
              <el-button size="small" text type="danger" @click="deleteGroup(g)">删除</el-button>
            </div>
          </li>
        </ul>
      </el-col>

      <!-- 右：项目 -->
      <el-col :span="18">
        <div class="panel-header">
          <span class="text-primary text-sm" style="font-weight:600">
            项目{{ selectedGroup ? ' · ' + selectedGroup.Name : '' }}
          </span>
          <el-button
              size="small"
              type="primary"
              :disabled="selectedGroupId === null"
              @click="openCreateProject">
            + 新建项目
          </el-button>
        </div>

        <el-skeleton v-if="loading" :rows="5" animated/>
        <div v-else-if="selectedGroupId === null" class="empty-wrap">
          <el-empty description="请选择左侧项目组"/>
        </div>
        <div v-else-if="projects.length === 0" class="empty-wrap">
          <el-empty description="该组下暂无项目"/>
        </div>
        <el-table v-else :data="projects" class="project-table">
          <el-table-column prop="Name" label="名称" min-width="160"/>
          <el-table-column prop="Remark" label="备注" min-width="220" show-overflow-tooltip>
            <template #default="{row}">
              <span v-if="row.Remark" class="text-xs truncate">{{ row.Remark }}</span>
              <span v-else class="text-placeholder">—</span>
            </template>
          </el-table-column>
          <el-table-column prop="LastAccessedAt" label="上次访问" width="180">
            <template #default="{row}">
              <span v-if="row.LastAccessedAt" class="text-xs">{{ formatTime(row.LastAccessedAt) }}</span>
              <span v-else class="text-placeholder">从未</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{row}">
              <div class="op-actions">
                <el-button v-if="row.Claimed" size="small" text type="primary" @click="openDetail(row)">详情</el-button>
                <el-button v-else size="small" type="success" @click="claimProject(row)">认领</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-col>
    </el-row>

    <!-- 项目组弹窗 -->
    <el-dialog v-model="showGroupModal" :title="isGroupEdit ? '编辑项目组' : '新建项目组'" width="600px">
      <el-form :model="groupForm" label-width="100px">
        <el-form-item label="组名称">
          <el-input v-model="groupForm.Name" placeholder="项目组名称"/>
        </el-form-item>
        <el-form-item label="排序">
          <el-input v-model.number="groupForm.OrderNum" type="number" placeholder="数字越小越靠前"/>
        </el-form-item>
        <el-form-item label="根目录">
          <el-input v-model="groupForm.AbsolutePath" placeholder="绝对路径，如 D:/code/mygroup"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="groupForm.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGroupModal = false">取消</el-button>
        <el-button type="primary" @click="saveGroup">保存</el-button>
      </template>
    </el-dialog>

    <!-- 项目弹窗 -->
    <el-dialog v-model="showProjectModal" :title="isProjectEdit ? '编辑项目' : '新建项目'" width="640px">
      <el-form :model="projectForm" label-width="100px">
        <el-form-item label="项目名称">
          <el-input v-model="projectForm.Name" placeholder="留空则取目录名"/>
        </el-form-item>
        <el-form-item label="绝对路径">
          <el-input v-model="projectForm.AbsolutePath" placeholder="与相对路径二选一，如 D:/code/mygroup/proj"/>
        </el-form-item>
        <el-form-item label="相对路径">
          <el-input v-model="projectForm.RelativePath" placeholder="相对所属组根目录，如 proj 或 sub/proj"/>
        </el-form-item>
        <el-form-item label="Git 地址">
          <el-input v-model="projectForm.GitURL" placeholder="可选 Git 仓库地址"/>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="projectForm.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showProjectModal = false">取消</el-button>
        <el-button type="primary" @click="saveProject">保存</el-button>
      </template>
    </el-dialog>

    <!-- 移动弹窗 -->
    <el-dialog v-model="showMoveModal" title="移动项目" width="600px">
      <el-form :model="moveForm" label-width="100px">
        <el-form-item label="目标项目组">
          <el-select v-model="moveForm.TargetGroupID" placeholder="选择目标项目组" style="width:100%">
            <el-option v-for="g in groups" :key="g.ID" :label="g.Name" :value="g.ID"/>
          </el-select>
        </el-form-item>
        <el-form-item label="目标相对路径">
          <el-input v-model="moveForm.TargetRelativePath" placeholder="相对目标组根目录的路径，如 proj 或 sub/proj"/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showMoveModal = false">取消</el-button>
        <el-button type="primary" :loading="moving" @click="doMove">移动</el-button>
      </template>
    </el-dialog>

    <!-- 详情弹窗 -->
    <el-dialog v-model="showDetail" title="项目详情" width="640px">
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
          <el-button size="small" @click="accessProject(detailItem!)">访问</el-button>
          <el-button size="small" type="primary" @click="openMove(detailItem!); showDetail = false">移动</el-button>
          <el-button size="small" @click="openEditProject(detailItem!); showDetail = false">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteProject(detailItem!); showDetail = false">删除</el-button>
        </template>
        <el-button v-else type="success" @click="claimProject(detailItem!); showDetail = false">认领</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: var(--space-sm);
  margin-bottom: var(--space-sm);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.group-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.group-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-sm);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  margin-bottom: var(--space-xs);
  cursor: pointer;
}

.group-item:hover {
  background: var(--el-fill-color-light);
}

.group-item.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.group-item-main {
  min-width: 0;
  flex: 1;
}

.group-item-ops {
  flex-shrink: 0;
  display: flex;
  gap: 0;
}

.project-table {
  width: 100%;
}

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
