<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'info' | 'success' | 'warning' | 'danger' }>(), { tone: 'info' })
</script>
<template><div :class="['notice-box', tone === 'warning' && 'warning', tone === 'danger' && 'danger']" role="status" data-ui-pattern="Notice"><slot /></div></template>
