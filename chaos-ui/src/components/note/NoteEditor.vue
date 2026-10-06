<script setup lang="ts">
// 笔记编辑器：左 Monaco 编辑，右实时 Markdown 预览；保存走 baseHash 乐观锁。
// 与外部修改冲突时（409）进入冲突态，提供「覆盖 / 重载」二选一，绝不自动覆盖。
import {computed, ref, watch} from 'vue'
import {ElMessage, ElMessageBox} from 'element-plus'
import MonacoEditor from '@/components/MonacoEditor.vue'
import MarkdownPreview from '@/components/MarkdownPreview.vue'
import type {Note, ConflictInfo} from '@/api/note'
import {noteApi} from '@/api/note'

const props = defineProps<{
  note: Note
  /** 当前已落盘的正文（用于判定 dirty 与还原） */
  content: string
  /** 乐观锁基准 hash */
  baseHash: string
  /** 预览格式：markdown / text */
  format: 'markdown' | 'text'
}>()

const emit = defineEmits<{
  (e: 'saved', payload: { hash: string; content: string }): void
  (e: 'reload'): void
  (e: 'renamed', note: Note): void
  (e: 'deleted'): void
}>()

const draft = ref(props.content)
const saving = ref(false)
const conflict = ref<ConflictInfo | null>(null)

// 切换笔记或外部重新加载内容时，重置草稿与冲突态。
watch(
    () => [props.note?.ID, props.content],
    () => {
      draft.value = props.content
      conflict.value = null
    },
)

const dirty = computed(() => draft.value !== props.content)

async function save(useHash: string) {
  if (!props.note || !dirty.value) return
  saving.value = true
  try {
    const res = await noteApi.saveContent(props.note.ID, draft.value, useHash)
    conflict.value = null
    ElMessage.success('已保存')
    emit('saved', {hash: res.contentHash, content: draft.value})
  } catch (e: any) {
    const cf = e?.response?.data?.conflict as ConflictInfo | undefined
    if (cf) {
      conflict.value = cf
      ElMessage.warning('文件已被外部修改（Obsidian / VS Code / git 等），请选择覆盖或重载')
    } else {
      ElMessage.error(e?.message || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

function onSave() {
  save(props.baseHash)
}

function onOverwrite() {
  if (!conflict.value) return
  save(conflict.value.currentHash)
}

function onReload() {
  conflict.value = null
  emit('reload')
}

function onReset() {
  draft.value = props.content
  conflict.value = null
}

async function onRename() {
  if (!props.note) return
  try {
    const {value: name} = await ElMessageBox.prompt('请输入新的文件名', '重命名', {
      inputValue: props.note.Name,
      inputValidator: (v) => (v && !/[/\\]/.test(v) ? true : '文件名不能包含路径分隔符'),
    })
    const res = await noteApi.renameNote(props.note.ID, name)
    ElMessage.success('已重命名')
    emit('renamed', res)
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '重命名失败')
  }
}

async function onDelete() {
  if (!props.note) return
  try {
    await ElMessageBox.confirm(`确定把「${props.note.Title || props.note.Name}」移入回收站？`, '删除', {
      type: 'warning',
    })
    await noteApi.deleteNote(props.note.ID)
    ElMessage.success('已移入回收站')
    emit('deleted')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.message || '删除失败')
  }
}
</script>

<template>
  <div class="note-editor">
    <div class="note-editor__toolbar">
      <div class="note-editor__title">
        <span class="note-editor__path">{{ note.RelPath }}</span>
      </div>
      <div class="note-editor__actions">
        <el-tag v-if="dirty" type="warning" size="small">未保存</el-tag>
        <el-tag v-else type="success" size="small">已同步</el-tag>
        <el-button size="small" :disabled="!dirty || saving" type="primary" @click="onSave">保存</el-button>
        <el-button size="small" :disabled="!dirty" @click="onReset">还原</el-button>
        <el-button size="small" @click="onRename">重命名</el-button>
        <el-button size="small" type="danger" @click="onDelete">删除</el-button>
      </div>
    </div>

    <el-alert
        v-if="conflict"
        class="note-editor__conflict"
        type="error"
        :closable="false"
        show-icon
        title="文件已被外部修改"
        :description="`当前磁盘版本更新（${conflict.diskUpdatedAt}），请选择「覆盖」以你的内容为准，或「重载」放弃本地改动。`"
    >
      <template #default>
        <div class="note-editor__conflict-actions">
          <el-button size="small" type="danger" :loading="saving" @click="onOverwrite">覆盖保存</el-button>
          <el-button size="small" :loading="saving" @click="onReload">重载最新</el-button>
        </div>
      </template>
    </el-alert>

    <div class="note-editor__body">
      <div class="note-editor__pane">
        <MonacoEditor v-model="draft" :language="format === 'markdown' ? 'markdown' : 'plaintext'"/>
      </div>
      <div class="note-editor__pane note-editor__preview">
        <MarkdownPreview :content="draft" :format="format"/>
      </div>
    </div>
  </div>
</template>

<style scoped>
.note-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.note-editor__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 3px 14px;
  border-bottom: 1px solid var(--el-border-color);
  background: var(--el-fill-color-lighter);
}

.note-editor__title {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.note-editor__name {
  font-weight: 600;
  font-size: 15px;
  line-height: 1.25;
}

.note-editor__path {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.25;
  word-break: break-all;
}

.note-editor__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.note-editor__conflict {
  margin: 8px 14px 0;
}

.note-editor__conflict-actions {
  margin-top: 8px;
  display: flex;
  gap: 8px;
}

.note-editor__body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.note-editor__pane {
  flex: 1;
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

.note-editor__preview {
  border-left: 1px solid var(--el-border-color);
}
</style>
