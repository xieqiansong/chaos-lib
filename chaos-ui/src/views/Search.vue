<script lang="ts" setup>
import {computed, ref, watch} from 'vue'
import type {SearchEngine} from './search-engine-dropdown'
import {ALL_ENGINES, ENGINE_CATEGORIES} from './search-engine-dropdown'

const emit = defineEmits<{
  (e: 'search-change', value: string): void
}>()

const input = ref('')
const selectedEngine = ref<SearchEngine>(ALL_ENGINES.find(e => e.name === 'google') || ALL_ENGINES[0])
const popoverVisible = ref(false)

watch(input, (newValue) => {
  emit('search-change', newValue)
})

const categorizedEngines = computed(() => {
  return Object.entries(ENGINE_CATEGORIES).map(([category, names]) => ({
    category,
    engines: names
        .map(name => ALL_ENGINES.find(e => e.name === name))
        .filter(Boolean) as SearchEngine[]
  }))
})

const selectEngine = (engine: SearchEngine) => {
  selectedEngine.value = engine
  popoverVisible.value = false
}

const search = () => {
  if (input.value.trim()) {
    const url = selectedEngine.value.url + encodeURIComponent(input.value.trim())
    window.open(url, '_blank')
  }
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter') {
    search()
  }
}
</script>

<template>
  <div class="search-bar">
    <el-input
        v-model="input"
        class="search-input"
        placeholder="请输入"
        clearable
        @keydown="handleKeydown"/>
    <el-popover
        v-model:visible="popoverVisible"
        placement="bottom-start"
        width="480"
        trigger="click"
        popper-class="engine-popover">
      <template #reference>
        <el-button class="engine-button">{{ selectedEngine.aliases?.[0] || selectedEngine.name }}</el-button>
      </template>
      <div class="popover-content">
        <div v-for="group in categorizedEngines" :key="group.category" class="engine-group">
          <div>{{ group.category === 'AI' ? 'AI' : group.category === 'SEARCH' ? '搜索' : '社交' }}</div>
          <div class="engine-grid">
            <div
                v-for="engine in group.engines"
                :key="engine.name"
                :class="['engine-item', { active: selectedEngine.name === engine.name }]"
                @click="selectEngine(engine)">
              <span>{{ engine.aliases?.[0] || engine.name }}</span>
            </div>
          </div>
        </div>
      </div>
    </el-popover>
    <el-button @click="search" class="search-button">搜索</el-button>
  </div>
</template>

<style scoped>
/* 弹性布局：输入框占满剩余空间，按钮按内容自适应宽度，
   窄窗口下不会像百分比列宽（el-col span）那样被压爆外溢 */
.search-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.search-input {
  flex: 1;
  min-width: 0;
}

.engine-button {
  flex-shrink: 0;
}

.search-button {
  flex-shrink: 0;
}

.popover-content {
}

.engine-group {
  margin-bottom: var(--space-lg);

  &:last-child {
    margin-bottom: 0;
  }
}

.engine-grid {
  display: flex;
  flex-wrap: wrap;
}

.engine-item {
  display: flex;
  align-items: center;
  padding: var(--space-sm) var(--space-lg);
  cursor: pointer;
}
</style>