<script setup lang="ts">
import AuditLogsRealView from './AuditLogsRealView.vue'
import AuditLogsView from './AuditLogsView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <AuditLogsRealView v-if="realMode" />
  <AuditLogsView v-else />
</template>
