<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

declare const monaco: any

const MONACO_PATH = '/assets/monaco/min/vs'

;(self as any).MonacoEnvironment = {
  getWorkerUrl(_moduleId: string, label: string) {
    return `${MONACO_PATH}/base/worker/workerMain.js`
  }
}

const props = withDefaults(defineProps<{
  original: string
  modelValue: string
  language?: string
}>(), {
  language: 'plaintext'
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const containerRef = ref<HTMLDivElement>()
let diffEditor: any = null
let originalModel: any = null
let modifiedModel: any = null
let isSelfUpdate = false

function createEditor() {
  if (!containerRef.value) return

  const lang = props.language || 'plaintext'

  originalModel = monaco.editor.createModel(props.original, lang)
  modifiedModel = monaco.editor.createModel(props.modelValue, lang)

  diffEditor = monaco.editor.createDiffEditor(containerRef.value, {
    automaticLayout: true,
    readOnly: false,
    originalEditable: false,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    lineNumbers: 'on',
    renderSideBySide: true,
    fontSize: 13,
    theme: 'vs',
    wordWrap: 'on',
  })

  diffEditor.setModel({
    original: originalModel,
    modified: modifiedModel,
  })

  modifiedModel.onDidChangeContent(() => {
    if (isSelfUpdate || !modifiedModel) return
    const val = modifiedModel.getValue()
    emit('update:modelValue', val)
  })
}

onMounted(() => {
  ;(window as any).require(['vs/editor/editor.main'], () => {
    createEditor()
  })
})

watch(() => props.original, (val) => {
  if (originalModel && val !== originalModel.getValue()) {
    originalModel.setValue(val)
  }
})

watch(() => props.modelValue, (val) => {
  if (modifiedModel && val !== modifiedModel.getValue()) {
    isSelfUpdate = true
    modifiedModel.setValue(val)
    isSelfUpdate = false
  }
})

watch(() => props.language, (lang) => {
  const l = lang || 'plaintext'
  if (originalModel) monaco.editor.setModelLanguage(originalModel, l)
  if (modifiedModel) monaco.editor.setModelLanguage(modifiedModel, l)
})

onUnmounted(() => {
  diffEditor?.dispose()
  originalModel?.dispose()
  modifiedModel?.dispose()
})
</script>

<template>
  <div ref="containerRef" class="monaco-diff-container"></div>
</template>

<style scoped>
.monaco-diff-container {
  width: 100%;
  height: 100%;
}
</style>