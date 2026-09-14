<script setup lang="ts">
import OrganizationRealView from './OrganizationRealView.vue'
import OrganizationView from './OrganizationView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <OrganizationRealView v-if="realMode" />
  <OrganizationView v-else />
</template>
