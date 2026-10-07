<script setup lang="ts">
// 通用「新建 / 编辑 / 查看」表单弹窗：字段由 fields 配置驱动，
// 与 DataTable 配套，使简单表的增改/查看界面也能配置化、零样板。
// mode 为 'view' 时所有控件禁用，底部仅保留「关闭」。
// 字段命名与 DataTableColumn 对齐：field（字段名）/ title（标签）。
import {computed} from 'vue'
import type {FormField} from '@/components/dataTable/types'
import {binarySummary, dataUrl, downloadBase64, isImageType, mimeOf} from '@/utils/binary'

const props = defineProps<{
  modelValue: boolean
  title?: string
  fields: FormField[]
  form: Record<string, any>
  saving?: boolean
  mode?: 'create' | 'edit' | 'view'
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  save: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

const readonly = computed(() => props.mode === 'view')

// 条件显隐：form[showIf.field] 满足 equals / notEquals 时才渲染该字段
function fieldVisible(f: FormField): boolean {
  if (!f.showIf) return true
  const cur = props.form[f.showIf.field]
  if (f.showIf.equals !== undefined) return cur === f.showIf.equals
  if (f.showIf.notEquals !== undefined) return cur !== f.showIf.notEquals
  return true
}

// binary 类型：查看时展示可读文本/预览+下载；编辑时直接编辑 base64
function binaryMime(f: FormField): string {
  return mimeOf(props.form[f.binaryDataTypeField ?? 'DataType'])
}

function binaryIsImage(f: FormField): boolean {
  return isImageType(props.form[f.binaryDataTypeField ?? 'DataType'])
}

// 查看模式：文本显示内容，二进制显示中性标签（base64 仅用于下载，不在弹窗里铺一屏）。
function binaryView(f: FormField): string {
  return binarySummary(String(props.form[f.field] ?? ''))
}

function downloadBinary(f: FormField) {
  const name = props.form[f.binaryFilename ?? 'Key']
  downloadBase64(String(props.form[f.field] ?? ''), name ? String(name) : 'download', binaryMime(f))
}
</script>

<template>
  <el-dialog v-model="visible" :title="title" width="800px">
    <el-form :model="form" label-width="100px">
      <el-row :gutter="16">
        <el-col v-for="f in fields" v-show="fieldVisible(f)" :key="f.field" :span="f.span ?? 24">
          <el-form-item
              :label="f.title"
              :required="f.required"
          >
            <el-input
                v-if="f.type === 'text'"
                v-model="form[f.field]"
                :disabled="readonly"
                :placeholder="f.placeholder"
            />
            <el-input
                v-else-if="f.type === 'password'"
                v-model="form[f.field]"
                type="password"
                show-password
                :disabled="readonly"
                :placeholder="f.placeholder"
            />
            <el-input
                v-else-if="f.type === 'textarea' || f.type === 'json'"
                v-model="form[f.field]"
                type="textarea"
                :rows="f.rows ?? 2"
                :disabled="readonly"
                :placeholder="f.placeholder"
            />
            <el-select
                v-else-if="f.type === 'select'"
                v-model="form[f.field]"
                :disabled="readonly"
                :placeholder="f.placeholder ?? '请选择'"
                style="width: 100%"
            >
              <el-option v-for="o in (f.options ?? [])" :key="o.value" :label="o.label" :value="o.value"/>
            </el-select>
            <el-input-number
                v-else-if="f.type === 'number'"
                v-model="form[f.field]"
                :min="f.min ?? 0"
                :precision="f.precision"
                :step="f.step"
                :disabled="readonly"
                controls-position="right"
            />
            <el-switch
                v-else-if="f.type === 'switch'"
                v-model="form[f.field]"
                :disabled="readonly"
            />
            <el-date-picker
                v-else-if="f.type === 'datetime'"
                v-model="form[f.field]"
                type="datetime"
                :value-format="f.valueFormat ?? 'YYYY-MM-DDTHH:mm:ssZ'"
                :disabled="readonly"
                :placeholder="f.placeholder ?? '可选'"
                style="width: 100%"
            />
            <!-- 二进制值：查看时给图片预览 + 文本/中性标签 + 下载；编辑/新建时编辑 base64 -->
            <div v-else-if="f.type === 'binary'" class="binary-field">
              <template v-if="readonly">
                <el-image
                    v-if="binaryIsImage(f)"
                    :src="dataUrl(form[f.field], binaryMime(f))"
                    :preview-src-list="[dataUrl(form[f.field], binaryMime(f))]"
                    fit="contain"
                    lazy
                    class="binary-preview"
                />
                <el-input :model-value="binaryView(f)" type="textarea" :rows="f.rows ?? 4" readonly/>
                <el-button size="small" type="primary" plain @click="downloadBinary(f)">下载二进制</el-button>
              </template>
              <el-input
                  v-else
                  v-model="form[f.field]"
                  type="textarea"
                  :rows="f.rows ?? 4"
                  :placeholder="f.placeholder"
              />
            </div>
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <!-- 默认底部按钮；业务需要额外操作（如详情页的跳转 / 删除）时用 #footer 插槽覆盖 -->
    <template #footer>
      <slot name="footer">
        <el-button v-if="readonly" @click="visible = false">关闭</el-button>
        <template v-else>
          <el-button @click="visible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="emit('save')">保存</el-button>
        </template>
      </slot>
    </template>
  </el-dialog>
</template>

<style scoped>
.binary-preview {
  width: 100%;
  max-height: 224px;
  margin-bottom: var(--space-sm);
  border: 1px solid var(--border-color, #d9d9d9);
  border-radius: 4px;
  background: #fff;
}

.binary-field {
  display: flex;
  flex-direction: column;
}

.binary-field :deep(.el-button) {
  width: fit-content;
  margin-top: var(--space-xs);
}
</style>
