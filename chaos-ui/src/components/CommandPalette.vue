<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'

interface CommandItem {
  key: string
  label: string
  alias: string
}

const props = defineProps<{
  visible: boolean
  commands: CommandItem[]
}>()

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'update:visible', v: boolean): void
}>()

const query = ref('')
const activeIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.commands
  return props.commands.filter(c =>
    c.key.toLowerCase().includes(q) ||
    c.label.toLowerCase().includes(q) ||
    c.alias.toLowerCase().includes(q)
  )
})

watch(() => props.visible, (v) => {
  if (v) {
    query.value = ''
    activeIndex.value = 0
    nextTick(() => inputRef.value?.focus())
  }
})

watch(filtered, () => { activeIndex.value = 0 })

function pick(i: number) {
  const item = filtered.value[i]
  if (item) emit('select', item.key)
}

function onInputEnter() {
  pick(activeIndex.value)
}

function move(delta: number) {
  const len = filtered.value.length
  if (!len) return
  activeIndex.value = (activeIndex.value + delta + len) % len
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') { e.preventDefault(); move(1) }
  else if (e.key === 'ArrowUp') { e.preventDefault(); move(-1) }
  else if (e.key === 'Enter') { e.preventDefault(); onInputEnter() }
  else if (e.key === 'Escape') { e.preventDefault(); emit('update:visible', false) }
}
</script>

<template>
  <div v-if="visible" class="cmdpalette-mask" @click.self="emit('update:visible', false)">
    <div class="cmdpalette term-glow">
      <div class="cmdpalette__input-row">
        <span class="term-prompt">user@chaos:~$</span>
        <input
          ref="inputRef"
          v-model="query"
          class="cmdpalette__input"
          placeholder="输入命令… (task / sdk / ll / env)"
          @keydown="onKey"
        />
        <span class="term-cursor"></span>
      </div>
      <ul class="cmdpalette__list">
        <li
          v-for="(c, i) in filtered"
          :key="c.key"
          :class="['cmdpalette__item', { active: i === activeIndex }]"
          @mouseenter="activeIndex = i"
          @click="pick(i)"
        >
          <span class="cmdpalette__alias">{{ c.alias }}</span>
          <span class="cmdpalette__label">{{ c.label }}</span>
        </li>
        <li v-if="!filtered.length" class="cmdpalette__empty">command not found: {{ query }}</li>
      </ul>
      <div class="cmdpalette__hint">↑↓ 选择 · ⏎ 执行 · esc 关闭</div>
    </div>
  </div>
</template>

<style scoped>
.cmdpalette-mask {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(3, 6, 3, 0.72);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 14vh;
  backdrop-filter: blur(2px);
}

.cmdpalette {
  width: min(560px, 92vw);
  background: var(--term-bg);
  border: 1px solid var(--term-border);
  box-shadow: 0 0 24px rgba(51, 255, 102, 0.18);
}

.cmdpalette__input-row {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-md) var(--space-lg);
  border-bottom: 1px solid var(--term-border);
}

.cmdpalette__input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  outline: none;
  color: var(--term-green);
  font-family: inherit;
  font-size: 0.95rem;
}

.cmdpalette__input::placeholder { color: var(--term-green-faint); opacity: 0.5; }

.cmdpalette__list {
  list-style: none;
  margin: 0;
  padding: var(--space-xs) 0;
  max-height: 50vh;
  overflow-y: auto;
}

.cmdpalette__item {
  display: flex;
  align-items: center;
  gap: var(--space-lg);
  padding: var(--space-sm) var(--space-lg);
  cursor: pointer;
  color: var(--term-green-faint);
}

.cmdpalette__item.active {
  background: rgba(51, 255, 102, 0.1);
  color: var(--term-green);
}

.cmdpalette__alias {
  color: var(--term-green);
  min-width: 5.5rem;
}

.cmdpalette__label {
  color: inherit;
  opacity: 0.85;
}

.cmdpalette__empty {
  padding: var(--space-md) var(--space-lg);
  color: var(--term-red);
}

.cmdpalette__hint {
  padding: var(--space-xs) var(--space-lg);
  font-size: 0.72rem;
  color: var(--term-green-faint);
  opacity: 0.6;
  border-top: 1px solid var(--term-border);
  text-align: right;
}
</style>
