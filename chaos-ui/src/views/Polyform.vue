<script setup lang="ts">
import {computed, onBeforeUnmount, reactive, ref, watch} from 'vue'
import {ElMessage} from 'element-plus'
import {
  FORMAT_LIST,
  FORMAT_META,
  SAMPLE,
  bytesToHex,
  bytesToHexCompact,
  defaultOptions,
  hexToBytes,
  reformat,
  runConvert,
  utf8Length,
  type FormatId,
  type PolyformOptions,
} from '@/utils/polyform'

// App.vue 会向所有页面注入 searchText，此处无过滤场景，仅接收避免透传告警
defineProps<{searchText?: string}>()

const from = ref<FormatId>('json')
const to = ref<FormatId>('yaml')
const fromMeta = computed(() => FORMAT_META[from.value])
const toMeta = computed(() => FORMAT_META[to.value])

const sourceText = ref('')
const outputText = ref('')
const outputBytes = ref<Uint8Array | null>(null)
const duration = ref(0)
const error = ref('')

const autoConvert = ref(true)
const optionsOpen = ref<string[]>([])
const options = reactive<PolyformOptions>(defaultOptions())

const fileRef = ref<HTMLInputElement>()
let timer: ReturnType<typeof setTimeout> | undefined

const sourcePlaceholder = computed(() =>
    from.value === 'cbor'
        ? 'CBOR 为二进制格式，请粘贴 HEX 文本（如 a2 64 6e 61 6d 65 65 63 68 61 6f 73）\n或点击「上传文件」直接载入 .cbor 文件'
        : `在此粘贴 ${fromMeta.value.label} 内容，或点击「上传文件」载入`
)

const inputBytes = computed(() => utf8Length(sourceText.value))
const outputSize = computed(() =>
    outputBytes.value ? outputBytes.value.length : utf8Length(outputText.value)
)

function doConvert() {
  error.value = ''
  if (!sourceText.value.trim()) {
    outputText.value = ''
    outputBytes.value = null
    duration.value = 0
    return
  }
  try {
    const input = from.value === 'cbor' ? hexToBytes(sourceText.value) : sourceText.value
    const result = runConvert(input, from.value, to.value, options)
    outputText.value = result.text
    outputBytes.value = result.bytes
    duration.value = result.duration
  } catch (e: any) {
    error.value = e?.message || String(e)
    outputText.value = ''
    outputBytes.value = null
  }
}

/** 自动转换：输入 / 格式 / 选项变化后防抖执行 */
function scheduleConvert() {
  if (!autoConvert.value) return
  if (timer) clearTimeout(timer)
  timer = setTimeout(doConvert, 250)
}

watch([sourceText, from, to], scheduleConvert)
watch(options, scheduleConvert, {deep: true})

function loadSample() {
  if (from.value === 'cbor') {
    try {
      const result = runConvert(SAMPLE.json, 'json', 'cbor', options)
      sourceText.value = result.bytes ? bytesToHexCompact(result.bytes) : result.text
    } catch (e: any) {
      error.value = e?.message || String(e)
      return
    }
  } else {
    sourceText.value = SAMPLE[from.value]
  }
  doConvert()
}

/** 用同格式往返一次，实现格式化 / 美化 */
function formatSource() {
  if (!sourceText.value.trim() || from.value === 'cbor') return
  error.value = ''
  try {
    sourceText.value = reformat(sourceText.value, from.value, options).text
    doConvert()
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

/** 交换输入输出：格式互换，内容互换后重新转换 */
function swapFormats() {
  const prevFrom = from.value
  from.value = to.value
  to.value = prevFrom
  const prevSource = sourceText.value
  sourceText.value = outputText.value
  outputText.value = prevSource
  doConvert()
}

function clearAll() {
  sourceText.value = ''
  outputText.value = ''
  outputBytes.value = null
  duration.value = 0
  error.value = ''
}

function pickFile() {
  fileRef.value?.click()
}

function guessFormat(name: string): FormatId | null {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  const hit = FORMAT_LIST.find(f => f.ext === ext)
  return hit ? hit.id : null
}

async function onFileChange(e: Event) {
  const el = e.target as HTMLInputElement
  const file = el.files?.[0]
  if (!file) return
  const guessed = guessFormat(file.name)
  if (guessed) from.value = guessed
  const bytes = new Uint8Array(await file.arrayBuffer())
  sourceText.value = from.value === 'cbor'
      ? bytesToHex(bytes)
      : new TextDecoder().decode(bytes)
  el.value = ''
  doConvert()
}

async function copyOutput() {
  const text = outputBytes.value ? bytesToHexCompact(outputBytes.value) : outputText.value
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动选择文本')
  }
}

function downloadOutput() {
  if (!outputText.value && !outputBytes.value) return
  const blob = outputBytes.value
      ? new Blob([outputBytes.value], {type: 'application/octet-stream'})
      : new Blob([outputText.value], {type: 'text/plain;charset=utf-8'})
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `chaos-convert.${toMeta.value.ext}`
  a.click()
  URL.revokeObjectURL(url)
}

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <div class="polyform">
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">格式转换</span>
      <el-tag size="small" type="info">polyform-tools</el-tag>
      <div class="section-actions">
        <span class="pf-auto">
          <span class="text-secondary text-xs">自动转换</span>
          <el-switch v-model="autoConvert" size="small" @change="autoConvert && doConvert()"/>
        </span>
        <el-button size="small" @click="loadSample">示例</el-button>
        <el-button size="small" @click="clearAll">清空</el-button>
        <el-button size="small" @click="swapFormats">⇄ 交换</el-button>
        <el-button size="small" type="primary" @click="doConvert">转换</el-button>
      </div>
    </div>

    <el-alert
        v-if="error"
        type="error"
        :title="error"
        show-icon
        closable
        class="mb-md"
        @close="error = ''"
    />

    <el-collapse v-model="optionsOpen" class="pf-collapse">
      <el-collapse-item title="转换选项" name="options">
        <div class="pf-options">
          <div v-if="from === 'json' || to === 'json'" class="pf-opt-block">
            <div class="pf-opt-title">JSON</div>
            <el-form-item label="缩进">
              <el-input-number v-model="options.json.indent" :min="0" :max="8" size="small" controls-position="right"/>
            </el-form-item>
          </div>

          <div v-if="from === 'yaml' || to === 'yaml'" class="pf-opt-block">
            <div class="pf-opt-title">YAML</div>
            <el-form-item label="版本">
              <el-select v-model="options.yaml.version" size="small" class="pf-opt-s">
                <el-option label="1.2" value="1.2"/>
                <el-option label="1.1" value="1.1"/>
              </el-select>
            </el-form-item>
            <el-form-item label="缩进">
              <el-input-number v-model="options.yaml.indent" :min="1" :max="8" size="small" controls-position="right"/>
            </el-form-item>
            <el-form-item label="折行宽度">
              <el-input-number v-model="options.yaml.lineWidth" :min="0" :max="200" size="small"
                               controls-position="right"/>
            </el-form-item>
          </div>

          <div v-if="from === 'csv' || to === 'csv'" class="pf-opt-block">
            <div class="pf-opt-title">CSV</div>
            <el-form-item label="分隔符">
              <el-input v-model="options.csv.delimiter" size="small" class="pf-opt-xs"/>
            </el-form-item>
            <el-form-item label="首行为表头">
              <el-switch v-model="options.csv.hasHeaders" size="small"/>
            </el-form-item>
            <el-form-item label="引号">
              <el-input v-model="options.csv.quote" size="small" class="pf-opt-xs"/>
            </el-form-item>
            <el-form-item label="转义符">
              <el-input v-model="options.csv.escape" size="small" class="pf-opt-xs"/>
            </el-form-item>
            <el-form-item label="换行">
              <el-select v-model="options.csv.recordDelimiter" size="small" class="pf-opt-s">
                <el-option label="LF (\n)" :value="'\n'"/>
                <el-option label="CRLF (\r\n)" :value="'\r\n'"/>
              </el-select>
            </el-form-item>
            <el-form-item label="防公式注入">
              <el-switch v-model="options.csv.escapeFormulas" size="small"/>
            </el-form-item>
          </div>

          <div v-if="from === 'xml' || to === 'xml'" class="pf-opt-block">
            <div class="pf-opt-title">XML</div>
            <span class="text-secondary text-xs">对象根最佳；数组根会生成 &lt;0&gt;/&lt;1&gt; 数字标签，建议先包一层对象根</span>
            <el-form-item label="属性前缀">
              <el-input v-model="options.xml.attributePrefix" size="small" class="pf-opt-xs"/>
            </el-form-item>
            <el-form-item label="格式化输出">
              <el-switch v-model="options.xml.format" size="small"/>
            </el-form-item>
            <el-form-item label="忽略声明">
              <el-switch v-model="options.xml.ignoreDeclaration" size="small"/>
            </el-form-item>
            <el-form-item label="忽略属性">
              <el-switch v-model="options.xml.ignoreAttributes" size="small"/>
            </el-form-item>
          </div>

          <div v-if="from === 'ini' || to === 'ini'" class="pf-opt-block">
            <div class="pf-opt-title">INI</div>
            <el-form-item label="数组分隔符">
              <el-input v-model="options.ini.delimiter" size="small" class="pf-opt-xs"/>
            </el-form-item>
            <el-form-item label="数组写法">
              <el-select v-model="options.ini.arrayMode" size="small" class="pf-opt-s">
                <el-option label="一行拼齐" value="join"/>
                <el-option label="重复键" value="repeat"/>
              </el-select>
            </el-form-item>
            <el-form-item label="类型推断">
              <el-switch v-model="options.ini.inferTypes" size="small"/>
            </el-form-item>
            <el-form-item label="段前空行">
              <el-switch v-model="options.ini.blankLineBeforeSection" size="small"/>
            </el-form-item>
          </div>

          <div v-if="from === 'cbor' || to === 'cbor'" class="pf-opt-block">
            <div class="pf-opt-title">CBOR</div>
            <span class="text-secondary text-xs">二进制格式，界面以每 16 字节一行的 HEX 展示；无附加选项</span>
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>

    <el-row :gutter="12" class="pf-row">
      <el-col :span="12">
        <div class="pf-panel">
          <div class="pf-panel-bar">
            <span class="term-prompt">in$</span>
            <el-select v-model="from" size="small" class="pf-select">
              <el-option v-for="f in FORMAT_LIST" :key="f.id" :label="f.label" :value="f.id"/>
            </el-select>
            <el-tag v-if="fromMeta.binary" size="small" type="warning">HEX</el-tag>
            <div class="pf-bar-actions">
              <el-button size="small" @click="pickFile">上传文件</el-button>
              <el-button size="small" :disabled="fromMeta.binary" @click="formatSource">格式化</el-button>
              <el-button size="small" @click="sourceText = ''">清空</el-button>
            </div>
          </div>
          <el-input
              v-model="sourceText"
              type="textarea"
              resize="none"
              spellcheck="false"
              class="pf-editor"
              :placeholder="sourcePlaceholder"
          />
          <div class="pf-panel-foot">
            <span>{{ sourceText.length }} 字符 / {{ inputBytes }} 字节</span>
            <span class="text-secondary">{{ fromMeta.hint }}</span>
          </div>
        </div>
      </el-col>

      <el-col :span="12">
        <div class="pf-panel">
          <div class="pf-panel-bar">
            <span class="term-prompt">out$</span>
            <el-select v-model="to" size="small" class="pf-select">
              <el-option v-for="f in FORMAT_LIST" :key="f.id" :label="f.label" :value="f.id"/>
            </el-select>
            <el-tag v-if="toMeta.binary" size="small" type="warning">HEX</el-tag>
            <div class="pf-bar-actions">
              <el-button size="small" @click="copyOutput">复制</el-button>
              <el-button size="small" @click="downloadOutput">下载</el-button>
            </div>
          </div>
          <el-input
              v-model="outputText"
              type="textarea"
              resize="none"
              readonly
              spellcheck="false"
              class="pf-editor"
              placeholder="转换结果"
          />
          <div class="pf-panel-foot">
            <span>{{ outputSize }} 字节 · {{ duration.toFixed(1) }} ms</span>
            <span class="text-secondary">{{ toMeta.hint }}</span>
          </div>
        </div>
      </el-col>
    </el-row>

    <input ref="fileRef" type="file" class="pf-file" @change="onFileChange"/>
  </div>
</template>

<style scoped>
.polyform {
  display: flex;
  flex-direction: column;
}

.pf-auto {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
}

.pf-collapse {
  margin-bottom: var(--space-md);
  border-top: 1px solid var(--term-border);
  border-bottom: 1px solid var(--term-border);
}

.pf-collapse :deep(.el-collapse-item__header),
.pf-collapse :deep(.el-collapse-item__wrap) {
  background: transparent;
  border-bottom-color: var(--term-border);
  color: var(--term-green-faint);
  font-size: var(--font-sm);
}

.pf-collapse :deep(.el-collapse-item__content) {
  padding-bottom: var(--space-sm);
}

.pf-options {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-lg);
}

.pf-opt-block {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-xs);
  padding: var(--space-sm) var(--space-md);
  border: 1px solid var(--term-border);
  border-radius: 2px;
}

.pf-opt-title {
  margin-right: var(--space-sm);
  font-weight: 600;
  color: var(--term-green);
  font-size: var(--font-sm);
}

.pf-opt-block :deep(.el-form-item) {
  margin-bottom: 0;
}

.pf-opt-block :deep(.el-form-item__label) {
  font-size: var(--font-xs);
  color: var(--el-text-color-secondary);
}

.pf-opt-xs {
  width: 4.5rem;
}

.pf-opt-s {
  width: 6.5rem;
}

.pf-row {
  flex: 1;
  min-height: 0;
}

.pf-panel {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--term-border);
  border-radius: 2px;
  background: var(--el-bg-color);
  overflow: hidden;
}

.pf-panel-bar {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-05) var(--space-sm);
  border-bottom: 1px solid var(--term-border);
  background: var(--el-fill-color-lighter);
  flex-wrap: wrap;
}

.pf-select {
  width: 6.5rem;
}

.pf-bar-actions {
  margin-left: auto;
  display: flex;
  gap: var(--space-xs);
}

.pf-editor {
  height: clamp(18rem, calc(100vh - 21rem), 60rem);
}

.pf-editor :deep(.el-textarea__inner) {
  height: 100%;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: var(--font-sm);
  line-height: 1.55;
  background: transparent;
  box-shadow: none;
  border: none;
  border-radius: 0;
}

.pf-panel-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  padding: var(--space-05) var(--space-sm);
  border-top: 1px solid var(--term-border);
  font-size: var(--font-xs);
  color: var(--el-text-color-secondary);
}

.pf-file {
  display: none;
}
</style>
