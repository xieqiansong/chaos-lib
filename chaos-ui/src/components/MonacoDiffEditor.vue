<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { theme } from '../theme'

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

function defineTerminalTheme() {
  monaco.editor.defineTheme('chaos-terminal', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: '', foreground: 'c9d1d9', background: '0d1117' },
      { token: 'comment', foreground: '8b949e', fontStyle: 'italic' },
      { token: 'keyword', foreground: 'ff7b72', fontStyle: 'bold' },
      { token: 'identifier', foreground: 'f0f6fc' },
      { token: 'string', foreground: 'a5d6ff' },
      { token: 'number', foreground: '79c0ff' },
      { token: 'operator', foreground: 'ff7b72' },
    ],
    colors: {
      'editor.background': '#0d1117',
      'editor.foreground': '#c9d1d9',
      'editorLineNumber.foreground': '#6e7681',
      'editorLineNumber.activeForeground': '#f0f6fc',
      'editor.selectionBackground': '#264f78',
      'editor.lineHighlightBackground': '#161b22',
      'editorCursor.foreground': '#58a6ff',
      'editor.inactiveSelectionBackground': '#264f7855',
      'editorSuggestWidget.background': '#161b22',
      'editorSuggestWidget.border': '#30363d',
      'editorSuggestWidget.foreground': '#c9d1d9',
      'editorSuggestWidget.highlightForeground': '#58a6ff',
      'editorSuggestWidget.selectedBackground': '#21262d',
      'diffEditor.insertedTextBackground': '#23863655',
      'diffEditor.removedTextBackground': '#f8514955',
      'diffEditor.insertedLineBackground': '#122814',
      'diffEditor.removedLineBackground': '#3d1216',
    },
  })
}

function definePaperTheme() {
  monaco.editor.defineTheme('chaos-paper', {
    base: 'vs',
    inherit: true,
    rules: [
      { token: '', foreground: '24292f', background: 'ffffff' },
      { token: 'comment', foreground: '4d5660', fontStyle: 'italic' },
      { token: 'keyword', foreground: 'cf222e', fontStyle: 'bold' },
      { token: 'identifier', foreground: '1f2328' },
      { token: 'string', foreground: '0a3069' },
      { token: 'number', foreground: '0550ae' },
      { token: 'operator', foreground: 'cf222e' },
    ],
    colors: {
      'editor.background': '#ffffff',
      'editor.foreground': '#24292f',
      'editorLineNumber.foreground': '#6e7781',
      'editorLineNumber.activeForeground': '#1f2328',
      'editor.selectionBackground': '#91c9ff',
      'editor.lineHighlightBackground': '#f6f8fa',
      'editorCursor.foreground': '#0969da',
      'editor.inactiveSelectionBackground': '#91c9ff55',
      'editorSuggestWidget.background': '#ffffff',
      'editorSuggestWidget.border': '#d1d9e0',
      'editorSuggestWidget.foreground': '#24292f',
      'editorSuggestWidget.highlightForeground': '#0969da',
      'editorSuggestWidget.selectedBackground': '#eaeef2',
      'diffEditor.insertedTextBackground': '#1a7f3722',
      'diffEditor.removedTextBackground': '#cf222e22',
      'diffEditor.insertedLineBackground': '#dcffe4',
      'diffEditor.removedLineBackground': '#ffeef0',
    },
  })
}

function currentEditorTheme() {
  return theme.value === 'paper' ? 'chaos-paper' : 'chaos-terminal'
}

function createEditor() {
  if (!containerRef.value) return

  defineTerminalTheme()
  definePaperTheme()

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
    theme: currentEditorTheme(),
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

watch(theme, () => {
  if (diffEditor) {
    diffEditor.updateOptions({theme: currentEditorTheme()})
  }
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