<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = defineProps<{
  original: string
  modelValue: string
  leftLabel?: string
  rightLabel?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const edited = ref(props.modelValue)

watch(() => props.modelValue, (val) => {
  if (val !== edited.value) {
    edited.value = val
  }
})

watch(edited, (val) => {
  emit('update:modelValue', val)
})

const originalLines = computed(() => props.original.split('\n'))
const editedLines = computed(() => edited.value.split('\n'))

function rowClass(i: number) {
  const ol = originalLines.value[i]
  const el = editedLines.value[i]
  if (ol == null && el != null) return 'line-added'
  if (ol != null && el == null) return 'line-del'
  if (ol !== el) return 'line-changed'
  return ''
}

const areaRef = ref<HTMLTextAreaElement>()
const leftAreaRef = ref<HTMLTextAreaElement>()
const rightNumsRef = ref<HTMLDivElement>()
const leftNumsRef = ref<HTMLDivElement>()

function syncScroll(source: 'left' | 'right') {
  const t = areaRef.value
  const l = leftAreaRef.value
  const rn = rightNumsRef.value
  const ln = leftNumsRef.value
  if (!t || !l || !rn || !ln) return

  if (source === 'right') {
    l.scrollTop = t.scrollTop
    rn.scrollTop = t.scrollTop
    ln.scrollTop = t.scrollTop
  } else {
    t.scrollTop = l.scrollTop
    rn.scrollTop = l.scrollTop
    ln.scrollTop = l.scrollTop
  }
}
</script>

<template>
  <div class="diff-wrap">
    <div class="pane pane-ro">
      <div class="pane-head">{{ leftLabel || '原始内容' }}</div>
      <div class="pane-body">
        <div ref="leftNumsRef" class="line-nums">
          <div v-for="(_, i) in originalLines" :key="i" class="ln">{{ i + 1 }}</div>
        </div>
        <textarea
            ref="leftAreaRef"
            class="edit"
            readonly
            :value="original"
            @scroll="syncScroll('left')"
        ></textarea>
      </div>
    </div>

    <div class="pane pane-ed">
      <div class="pane-head">{{ rightLabel || '编辑区' }}</div>
      <div class="pane-body">
        <div ref="rightNumsRef" class="line-nums">
          <div
              v-for="(_, i) in editedLines"
              :key="i"
              class="ln"
              :class="rowClass(i)"
          >{{ i + 1 }}</div>
        </div>
        <textarea
            ref="areaRef"
            class="edit"
            v-model="edited"
            @scroll="syncScroll('right')"
            spellcheck="false"
        ></textarea>
      </div>
    </div>
  </div>
</template>

<style scoped>
.diff-wrap {
  display: flex;
  gap: 12px;
  height: 100%;
}
.pane {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  display: flex;
  flex-direction: column;
}
.pane-head {
  padding: 6px 10px;
  font-size: var(--el-font-size-extra-small);
  color: var(--el-text-color-secondary);
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  flex-shrink: 0;
}
.pane-body {
  display: flex;
  overflow: hidden;
  flex: 1;
}
.line-nums {
  width: 42px;
  flex-shrink: 0;
  padding: 8px 0;
  text-align: right;
  user-select: none;
  font-family: Consolas, 'Courier New', monospace;
  font-size: var(--el-font-size-small);
  line-height: 20px;
  color: var(--el-text-color-secondary);
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  overflow-y: scroll;
  scrollbar-width: none;
}
.line-nums::-webkit-scrollbar {
  display: none;
}
.ln {
  padding-right: 8px;
}
.ln.line-added   { background: var(--el-color-success-light-9); color: var(--el-color-success); }
.ln.line-del     { background: var(--el-color-danger-light-9); color: var(--el-color-danger); }
.ln.line-changed { background: var(--el-color-warning-light-9); color: var(--el-color-warning); }

.edit {
  flex: 1;
  resize: none;
  border: none;
  outline: none;
  padding: 8px 10px;
  font-family: Consolas, 'Courier New', monospace;
  font-size: var(--el-font-size-small);
  line-height: 20px;
  background: transparent;
}
.pane-ro .edit {
  cursor: default;
  color: var(--el-text-color-regular);
}
</style>