<script setup lang="ts">
import RolesView from './RolesView.vue'
import RolesRealView from './RolesRealView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <RolesRealView v-if="realMode" />
  <RolesView v-else />
</template>
