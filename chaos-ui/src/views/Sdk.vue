<template>
    <div class="sdk-container">
        <div class="sdk-header">
            <h2>SDK 版本管理</h2>
            <el-button type="primary" :icon="Setting" @click="openManage">管理来源</el-button>
        </div>

        <el-alert
            v-if="error"
            :title="error"
            type="error"
            show-icon
            :closable="false"
            style="margin-bottom: 16px"
        />

        <el-row :gutter="16">
            <el-col v-for="(info, type) in sdkList" :key="type" :xs="24" :sm="12" :md="8" :lg="6">
                <el-card class="sdk-card" shadow="hover">
                    <template #header>
                        <div class="card-header">
                            <span class="sdk-type">{{ type }}</span>
                            <el-tag
                                v-if="!defEnabled(type)"
                                size="small"
                                type="info"
                            >已禁用</el-tag>
                        </div>
                    </template>

                    <!-- 来源列表 -->
                    <div class="sources">
                        <div
                            v-for="(src, idx) in defSources(type)"
                            :key="idx"
                            class="source-item"
                        >
                            <el-tag
                                size="small"
                                :type="src.kind === 'repo' ? 'primary' : 'warning'"
                            >{{ src.kind }}</el-tag>
                            <span class="source-root" :title="src.root">{{ src.root }}</span>
                        </div>
                    </div>

                    <!-- 版本标签 -->
                    <div class="version-tags">
                        <el-tag
                            v-for="v in info.versionList"
                            :key="v"
                            class="version-tag"
                            :type="v === info.currentVersion ? 'success' : 'info'"
                            :effect="v === info.currentVersion ? 'dark' : 'plain'"
                            :class="{ clickable: canSwitch(type), disabled: !canSwitch(type) }"
                            @click="canSwitch(type) ? switchVersion(type, v) : onSingleClick(type)"
                        >
                            {{ v }}
                        </el-tag>
                        <span v-if="info.versionList.length === 0" class="empty-hint">暂无版本</span>
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- 管理来源对话框 -->
        <el-dialog v-model="manageVisible" title="SDK 来源管理" width="720px">
            <div class="manage-toolbar">
                <el-button type="primary" :icon="Plus" @click="addDef">新增 SDK 类型</el-button>
            </div>

            <el-table :data="defs" stripe style="width: 100%">
                <el-table-column prop="Name" label="类型" width="120" />
                <el-table-column label="来源" min-width="260">
                    <template #default="{ row }">
                        <div v-for="(s, i) in row.Sources" :key="i" class="def-source">
                            <el-tag size="small" :type="s.kind === 'repo' ? 'primary' : 'warning'">
                                {{ s.kind }}
                            </el-tag>
                            <span class="def-root">{{ s.root }}</span>
                        </div>
                    </template>
                </el-table-column>
                <el-table-column label="启用" width="80">
                    <template #default="{ row }">
                        <el-switch :model-value="row.Enabled" disabled />
                    </template>
                </el-table-column>
                <el-table-column label="操作" width="160">
                    <template #default="{ row }">
                        <el-button size="small" @click="editDef(row)">编辑</el-button>
                        <el-button size="small" type="danger" @click="removeDef(row)">删除</el-button>
                    </template>
                </el-table-column>
            </el-table>
        </el-dialog>

        <!-- 编辑/新增表单对话框 -->
        <el-dialog
            v-model="formVisible"
            :title="editingDef ? '编辑 ' + form.Name : '新增 SDK 类型'"
            width="640px"
        >
            <el-form :model="form" label-width="90px">
                <el-form-item label="类型名" required>
                    <el-input v-model="form.Name" :disabled="!!editingDef" placeholder="如 jdk / maven" />
                </el-form-item>

                <el-form-item label="启用">
                    <el-switch v-model="form.Enabled" />
                </el-form-item>

                <el-form-item label="来源">
                    <div class="source-form-list">
                        <div v-for="(s, i) in form.Sources" :key="i" class="source-form-item">
                            <el-select v-model="s.kind" style="width: 110px">
                                <el-option label="仓库 repo" value="repo" />
                                <el-option label="单版本 single" value="single" />
                            </el-select>
                            <el-input v-model="s.root" placeholder="绝对路径，如 D:\opt\jdk" style="flex: 1" />
                            <el-button type="danger" :icon="Delete" circle @click="removeSource(i)" />
                        </div>
                        <el-button :icon="Plus" plain @click="addSource">添加来源</el-button>
                    </div>
                </el-form-item>

                <el-form-item label="备注">
                    <el-input v-model="form.Note" type="textarea" :rows="2" />
                </el-form-item>
            </el-form>

            <template #footer>
                <el-button @click="formVisible = false">取消</el-button>
                <el-button type="primary" @click="submitDef">保存</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { Setting, Plus, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
    getSdkVersions,
    updateSdkVersion,
    getSdkDefs,
    createSdkDef,
    updateSdkDef,
    deleteSdkDef,
    type SdkSource,
    type SdkSourceItem,
} from '@/utils/api'

interface SdkListEntry {
    currentVersion: string
    versionList: string[]
}

const sdkList = ref<Record<string, SdkListEntry>>({})
const defs = ref<SdkSource[]>([])
const error = ref('')

const manageVisible = ref(false)
const formVisible = ref(false)
const editingDef = ref<SdkSource | null>(null)
const form = reactive<{
    Name: string
    Sources: SdkSourceItem[]
    Enabled: boolean
    Note: string
}>({
    Name: '',
    Sources: [],
    Enabled: true,
    Note: '',
})

function defMap(): Record<string, SdkSource> {
    const m: Record<string, SdkSource> = {}
    for (const d of defs.value) m[d.Name] = d
    return m
}

function defSources(type: string): SdkSourceItem[] {
    return defMap()[type]?.Sources ?? []
}

function defEnabled(type: string): boolean {
    const d = defMap()[type]
    return d ? d.Enabled : true
}

// 仅含 single 来源的类型不可切换；含至少一个 repo 则可切换
function canSwitch(type: string): boolean {
    const sources = defSources(type)
    if (sources.length === 0) return true // 无来源信息（defs 未加载）时保守允许
    return sources.some((s) => s.kind === 'repo')
}

async function switchVersion(type: string, version: string) {
    try {
        await updateSdkVersion(type, version)
        await loadVersions()
        ElMessage.success(`已切换到 ${type} ${version}`)
    } catch (e: any) {
        const msg = e?.message || ''
        if (msg.includes('400') || msg.includes('不支持')) {
            ElMessage.warning('该来源不支持切换')
        } else {
            ElMessage.error('切换失败: ' + msg)
        }
    }
}

function onSingleClick(type: string) {
    ElMessage.info(`${type} 为单版本来源，不支持切换`)
}

async function loadVersions() {
    try {
        sdkList.value = await getSdkVersions()
    } catch (e: any) {
        error.value = '加载 SDK 版本失败: ' + (e?.message || '')
    }
}

async function loadDefs() {
    try {
        defs.value = await getSdkDefs()
    } catch (e: any) {
        // defs 失败不阻塞只读视图
        console.warn('加载 SDK 来源失败:', e)
    }
}

function openManage() {
    manageVisible.value = true
}

function addDef() {
    editingDef.value = null
    form.Name = ''
    form.Sources = [{ kind: 'repo', root: '' }]
    form.Enabled = true
    form.Note = ''
    manageVisible.value = false
    formVisible.value = true
}

function editDef(row: SdkSource) {
    editingDef.value = row
    form.Name = row.Name
    form.Sources = row.Sources.map((s) => ({ ...s }))
    form.Enabled = row.Enabled
    form.Note = row.Note
    manageVisible.value = false
    formVisible.value = true
}

function addSource() {
    form.Sources.push({ kind: 'repo', root: '' })
}

function removeSource(i: number) {
    form.Sources.splice(i, 1)
}

async function submitDef() {
    if (!form.Name) {
        ElMessage.warning('请填写类型名')
        return
    }
    if (form.Sources.some((s) => !s.root)) {
        ElMessage.warning('每个来源都需要填写 root 路径')
        return
    }
    try {
        if (editingDef.value) {
            await updateSdkDef(editingDef.value.Name, {
                Sources: form.Sources,
                Enabled: form.Enabled,
                Note: form.Note,
            })
        } else {
            await createSdkDef({
                Name: form.Name,
                Sources: form.Sources,
                Enabled: form.Enabled,
                Note: form.Note,
            })
        }
        ElMessage.success('保存成功')
        formVisible.value = false
        await loadDefs()
        await loadVersions()
    } catch (e: any) {
        ElMessage.error('保存失败: ' + (e?.message || ''))
    }
}

async function removeDef(row: SdkSource) {
    try {
        await ElMessageBox.confirm(`确认删除 SDK 类型 "${row.Name}"？`, '删除确认', {
            type: 'warning',
        })
    } catch {
        return
    }
    try {
        await deleteSdkDef(row.Name)
        ElMessage.success('已删除')
        await loadDefs()
        await loadVersions()
    } catch (e: any) {
        ElMessage.error('删除失败: ' + (e?.message || ''))
    }
}

onMounted(() => {
    loadVersions()
    loadDefs()
})
</script>

<style scoped>
.sdk-container {
    padding: 16px;
}
.sdk-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
}
.card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
}
.sdk-type {
    font-weight: 600;
    font-size: 16px;
}
.sources {
    margin-bottom: 12px;
}
.source-item {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
    font-size: 13px;
}
.source-root {
    color: #909399;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.version-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}
.version-tag.clickable {
    cursor: pointer;
}
.version-tag.disabled {
    cursor: not-allowed;
    opacity: 0.7;
}
.empty-hint {
    color: #c0c4cc;
    font-size: 13px;
}
.manage-toolbar {
    margin-bottom: 12px;
}
.def-source {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
}
.def-root {
    font-size: 13px;
    color: #606266;
}
.source-form-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
}
.source-form-item {
    display: flex;
    align-items: center;
    gap: 8px;
}
</style>
