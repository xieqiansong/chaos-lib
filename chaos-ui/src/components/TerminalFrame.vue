<script setup lang="ts">
// 终端窗口外壳：标题栏（user@chaos:~$ <title>）+ 装饰圆点 + 荧光边框
withDefaults(defineProps<{
  title?: string
  prompt?: string
  flat?: boolean
}>(), {
  title: 'chaos',
  prompt: 'user@chaos',
  flat: false,
})
</script>

<template>
  <div class="term-frame" :class="{ 'term-frame--flat': flat }">
    <div class="term-titlebar">
      <span class="term-dots">
        <span class="term-dot red"></span>
        <span class="term-dot amber"></span>
        <span class="term-dot green"></span>
      </span>
      <span class="term-prompt">{{ prompt }}:~$</span>
      <span class="term-frame__title truncate">{{ title }}</span>
    </div>
    <div class="term-frame__body">
      <slot/>
    </div>
  </div>
</template>

<style scoped>
.term-frame {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--term-bg);
  border: 1px solid var(--term-border);
  box-shadow: 0 0 0 1px rgba(51, 255, 102, 0.06),
  inset 0 0 18px rgba(51, 255, 102, 0.04);
  overflow: hidden;
}

.term-frame--flat {
  border: none;
  box-shadow: none;
  background: transparent;
}

.term-frame__title {
  color: var(--term-green);
}

.term-frame__body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
