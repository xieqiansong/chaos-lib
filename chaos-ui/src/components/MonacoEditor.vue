<script setup lang="ts">
// 单栏 Monaco 编辑器封装（笔记编辑用）。主题与 worker 引导沿用 MonacoDiffEditor 既定方式。
import {ref, onMounted, onUnmounted, watch} from 'vue'
import {theme} from '../theme'

declare const monaco: any

const MONACO_PATH = '/assets/monaco/min/vs'

;(self as any).MonacoEnvironment = {
  getWorkerUrl(_moduleId: string, _label: string) {
    return `${MONACO_PATH}/base/worker/workerMain.js`
  },
}

const props = withDefaults(defineProps<{
  modelValue: string
  language?: string
  readOnly?: boolean
}>(), {
  language: 'markdown',
  readOnly: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const containerRef = ref<HTMLDivElement>()
let editor: any = null
let model: any = null
let isSelfUpdate = false

function defineTerminalTheme() {
  monaco.editor.defineTheme('chaos-terminal', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      {token: '', foreground: 'c9d1d9', background: '0d1117'},
      {token: 'comment', foreground: '8b949e', fontStyle: 'italic'},
      {token: 'keyword', foreground: 'ff7b72', fontStyle: 'bold'},
      {token: 'identifier', foreground: 'f0f6fc'},
      {token: 'string', foreground: 'a5d6ff'},
      {token: 'number', foreground: '79c0ff'},
      {token: 'operator', foreground: 'ff7b72'},
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
    },
  })
}

function definePaperTheme() {
  monaco.editor.defineTheme('chaos-paper', {
    base: 'vs',
    inherit: true,
    rules: [
      {token: '', foreground: '24292f', background: 'ffffff'},
      {token: 'comment', foreground: '4d5660', fontStyle: 'italic'},
      {token: 'keyword', foreground: 'cf222e', fontStyle: 'bold'},
      {token: 'identifier', foreground: '1f2328'},
      {token: 'string', foreground: '0a3069'},
      {token: 'number', foreground: '0550ae'},
      {token: 'operator', foreground: 'cf222e'},
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

  const lang = props.language || 'markdown'
  model = monaco.editor.createModel(props.modelValue, lang)
  editor = monaco.editor.create(containerRef.value, {
    model,
    automaticLayout: true,
    readOnly: props.readOnly,
    minimap: {enabled: false},
    scrollBeyondLastLine: false,
    lineNumbers: 'on',
    fontSize: 13,
    theme: currentEditorTheme(),
    wordWrap: 'on',
  })

  editor.onDidChangeModelContent(() => {
    if (isSelfUpdate || !model) return
    emit('update:modelValue', model.getValue())
  })
}

onMounted(() => {
  ;(window as any).require(['vs/editor/editor.main'], () => createEditor())
})

watch(() => props.modelValue, (val) => {
  if (model && val !== model.getValue()) {
    isSelfUpdate = true
    model.setValue(val)
    isSelfUpdate = false
  }
})

watch(() => props.language, (lang) => {
  if (model) monaco.editor.setModelLanguage(model, lang || 'markdown')
})

watch(theme, () => {
  if (editor) editor.updateOptions({theme: currentEditorTheme()})
})

onUnmounted(() => {
  editor?.dispose()
  model?.dispose()
})
</script>

<template>
  <div ref="containerRef" class="monaco-editor-container"></div>
</template>

<style scoped>
.monaco-editor-container {
  width: 100%;
  height: 100%;
}
</style>
