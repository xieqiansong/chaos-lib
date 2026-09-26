<script setup lang="ts">
// 通用「新建 / 编辑 / 查看」表单弹窗：字段由 fields 配置驱动，
// 与 DataTable 配套，使简单表的增改/查看界面也能配置化、零样板。
// mode 为 'view' 时所有控件禁用，底部仅保留「关闭」。
// 字段命名与 DataTableColumn 对齐：field（字段名）/ title（标签）。
import {computed} from 'vue'
import type {FormField} from '@/components/dataTable/types'

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
</script>

<template>
  <el-dialog v-model="visible" :title="title" width="50rem">
    <el-form :model="form" label-width="6.25rem">
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
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <template #footer>
      <el-button v-if="readonly" @click="visible = false">关闭</el-button>
      <template v-else>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="emit('save')">保存</el-button>
      </template>
    </template>
  </el-dialog>
</template>
