<script setup lang="ts">
import CompanyRealView from './CompanyRealView.vue'
import CompanyView from './CompanyView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <CompanyRealView v-if="realMode" />
  <CompanyView v-else />
</template>
