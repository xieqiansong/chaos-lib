<script setup lang="ts">
import { ref } from 'vue'
import DiffEditor from '../components/DiffEditor.vue'

const original = ref(`line1
line2
line3
line4`)

const current = ref(original.value)
</script>

<template>
  <div class="example-container">
    <DiffEditor
        v-model="current"
        :original="original"
        left-label="原始内容（只读）"
        right-label="编辑区"
    />
  </div>
</template>

<style scoped>
.example-container {
  height: 70vh;
  padding: 1.25rem;
}
</style>