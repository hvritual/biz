<script setup lang="ts">
import { computed } from 'vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import type { PlanVersionDTO } from '@/services/commercial/platformCommercial'
const props = defineProps<{ target: PlanVersionDTO }>()
const modules = computed(() => props.target.terms?.modules ?? [])
</script>
<template>
  <section data-plan-target-details class="details">
    <h3>{{ target.name || backendTermLabel('plan', target.planCode) }}</h3>
    <p>仅展示该已发布版本的套餐条款，确认前会再次校验。</p>
    <article v-for="module in modules" :key="module.moduleCode"><strong>{{ backendTermLabel('module', module.moduleCode) }}</strong><span>{{ module.capabilityCodes?.length ?? 0 }} 项能力 · {{ module.quotas?.length ?? 0 }} 项额度</span><small v-for="capability in module.capabilityCodes ?? []" :key="capability">{{ backendTermLabel('entitlementKey', capability) }}</small><small v-for="quota in module.quotas ?? []" :key="quota.key">{{ backendTermLabel('entitlementKey', quota.key) }}：{{ quota.unlimited ? '不限量' : quota.value }}</small></article>
  </section>
</template>
<style scoped>.details{margin-top:12px;padding:14px;border:1px solid var(--color-border);border-radius:10px}.details h3{margin:0}.details p,.details span{color:var(--color-text-muted);font-size:12px}.details article{display:grid;gap:4px;margin-top:10px;padding:10px;background:var(--color-surface-muted);border-radius:8px}</style>
