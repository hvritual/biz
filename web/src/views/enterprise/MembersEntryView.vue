<script setup lang="ts">
import MembersView from './MembersView.vue'
import MembersRealView from './MembersRealView.vue'

const realMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
</script>

<template>
  <MembersRealView v-if="realMode" />
  <MembersView v-else />
</template>
