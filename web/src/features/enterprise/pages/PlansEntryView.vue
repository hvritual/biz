<script setup lang="ts">
import PlansRealView from './PlansRealView.vue'
import PlansView from './PlansView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <PlansRealView v-if="realMode" />
  <PlansView v-else />
</template>
