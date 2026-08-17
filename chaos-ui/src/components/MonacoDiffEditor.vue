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
      { token: '', foreground: '8de8a3', background: '0a0e0a' },
      { token: 'comment', foreground: '4f9d68', fontStyle: 'italic' },
      { token: 'keyword', foreground: '33ff66', fontStyle: 'bold' },
      { token: 'identifier', foreground: 'b8ffc8' },
      { token: 'string', foreground: 'ffb000' },
      { token: 'number', foreground: '2de2e6' },
      { token: 'operator', foreground: '33ff66' },
    ],
    colors: {
      'editor.background': '#0a0e0a',
      'editor.foreground': '#8de8a3',
      'editorLineNumber.foreground': '#4f9d68',
      'editorLineNumber.activeForeground': '#33ff66',
      'editor.selectionBackground': '#1d9b3e55',
      'editor.lineHighlightBackground': '#111811',
      'editorCursor.foreground': '#33ff66',
      'editor.inactiveSelectionBackground': '#1d9b3e33',
      'editorSuggestWidget.background': '#0d120d',
      'editorSuggestWidget.border': '#1f3d28',
      'editorSuggestWidget.foreground': '#8de8a3',
      'editorSuggestWidget.highlightForeground': '#33ff66',
      'editorSuggestWidget.selectedBackground': '#1f3d28',
      'diffEditor.insertedTextBackground': '#33ff6622',
      'diffEditor.removedTextBackground': '#ff5f5622',
      'diffEditor.insertedLineBackground': '#0e3a18',
      'diffEditor.removedLineBackground': '#3a0e0e',
    },
  })
}

function definePaperTheme() {
  monaco.editor.defineTheme('chaos-paper', {
    base: 'vs',
    inherit: true,
    rules: [
      { token: '', foreground: '5c4f38', background: 'f8f3e6' },
      { token: 'comment', foreground: 'a29578', fontStyle: 'italic' },
      { token: 'keyword', foreground: '7a8a45', fontStyle: 'bold' },
      { token: 'identifier', foreground: '4a3f2d' },
      { token: 'string', foreground: 'c9952f' },
      { token: 'number', foreground: '4a7f7f' },
      { token: 'operator', foreground: '7a8a45' },
    ],
    colors: {
      'editor.background': '#f8f3e6',
      'editor.foreground': '#5c4f38',
      'editorLineNumber.foreground': '#a29578',
      'editorLineNumber.activeForeground': '#7a8a45',
      'editor.selectionBackground': '#d6c9a866',
      'editor.lineHighlightBackground': '#efe8d2',
      'editorCursor.foreground': '#7a8a45',
      'editor.inactiveSelectionBackground': '#d6c9a844',
      'editorSuggestWidget.background': '#fdf9ee',
      'editorSuggestWidget.border': '#d6c9a8',
      'editorSuggestWidget.foreground': '#5c4f38',
      'editorSuggestWidget.highlightForeground': '#7a8a45',
      'editorSuggestWidget.selectedBackground': '#e6dcc2',
      'diffEditor.insertedTextBackground': '#7a8a4522',
      'diffEditor.removedTextBackground': '#b5653f22',
      'diffEditor.insertedLineBackground': '#eae4ce',
      'diffEditor.removedLineBackground': '#f0ddd3',
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