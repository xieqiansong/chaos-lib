<script lang="ts" setup>
import {ref} from 'vue'
import {saveBrowserHistory} from "../background/browser-history-backup";

const syncing = ref(false)

async function syncHistory() {
  syncing.value = true
  try {
    await saveBrowserHistory()
  } catch (e) {
    console.error('syncHistory failed', e)
  } finally {
    syncing.value = false
  }
}
</script>

<template>
  <div class="popup-container">
    <el-button type="primary" :loading="syncing" @click="syncHistory">
      同步历史记录
    </el-button>
  </div>
</template>

<style scoped>
.popup-container {
  width: 200px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.popup-container :deep(.el-button) {
  width: 100%;
  margin-left: 0;
}
</style>
