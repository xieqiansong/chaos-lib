<script lang="ts" setup>
import {ref} from 'vue'
import {saveBrowserHistory} from "../background/browser-history-backup";
import {saveBookmarks} from "@/entrypoints/background/bookmark-backup";

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

async function syncBookmarks() {
  try {
    await saveBookmarks()
  } catch (e) {
    console.error('syncBookmarks failed', e)
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
    <el-button type="primary" :loading="syncing" @click="syncBookmarks">
      同步书签
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
