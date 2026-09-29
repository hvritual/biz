<script setup lang="ts">
import { computed } from 'vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import type { PlanVersionDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{ target: PlanVersionDTO }>()

const modules = computed(() => (props.target.terms?.modules ?? []).map((module) => ({
  id: module.moduleCode,
  label: backendTermLabel('module', module.moduleCode),
  capabilityCount: module.capabilityCodes?.length ?? 0,
  quotaCount: module.quotas?.length ?? 0,
  fieldCount: module.fields?.length ?? 0,
  capabilities: module.capabilityCodes ?? [],
  quotas: module.quotas ?? [],
  fields: module.fields ?? [],
})))

function priceLabel() {
  const amount = Number(props.target.terms?.amountMinor)
  const currency = String(props.target.terms?.currency ?? '')
  if (!String(props.target.terms?.priceRef ?? '').trim()) return '免费开通'
  if (!Number.isSafeInteger(amount) || amount < 1 || !/^[A-Z]{3}$/.test(currency)) return '价格待确认'
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(amount / 100)
}

function validityLabel() {
  const terms = props.target.terms
  if (terms?.validityMode === 'fixed_days') return `${terms.validityDays} 天`
  return terms?.validityMode === 'unlimited' ? '长期有效' : terms?.validityMode || '—'
}

function quotaLabel(quota: { key: string, unlimited: boolean, value: string | number }) {
  return `${backendTermLabel('entitlementKey', quota.key)}：${quota.unlimited ? '不限量' : quota.value}`
}

function fieldLabel(field: { key: string, action: string, mode: string }) {
  return `${backendTermLabel('entitlementKey', field.key)} · ${field.action} → ${field.mode}`
}
</script>

<template>
  <section class="selected-plan-details" data-plan-target-details>
    <div class="row-between">
      <div><span class="field-label">已选套餐权益</span><h3>{{ target.name || backendTermLabel('plan', target.planCode) }}</h3></div>
      <span class="comparison-price">{{ priceLabel() }}</span>
    </div>
    <p class="muted">{{ validityLabel() }} · 仅展示该已发布版本的套餐条款，确认前会再次校验。</p>
    <div class="selected-module-grid">
      <article v-for="module in modules" :key="module.id" class="selected-module">
        <strong>{{ module.label }}</strong>
        <span>{{ module.capabilityCount }} 项能力 · {{ module.quotaCount }} 项额度 · {{ module.fieldCount }} 项字段策略</span>
        <div v-if="module.capabilities.length" class="term-list"><small v-for="capability in module.capabilities" :key="capability">{{ backendTermLabel('entitlementKey', capability) }}</small></div>
        <div v-if="module.quotas.length" class="term-list"><small v-for="quota in module.quotas" :key="quota.key">{{ quotaLabel(quota) }}</small></div>
        <div v-if="module.fields.length" class="term-list"><small v-for="field in module.fields" :key="`${field.key}:${field.action}`">{{ fieldLabel(field) }}</small></div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.selected-plan-details { margin-top: 12px; padding: 14px; border: 1px solid var(--color-border); border-radius: 10px; }
.selected-plan-details h3 { margin: 4px 0 0; font-size: 14px; }
.selected-plan-details > .muted { margin: 7px 0 0; }
.field-label { color: var(--color-text-muted); font-size: 12px; font-weight: 600; }
.comparison-price { flex: 0 0 auto; font-weight: 700; color: var(--color-primary); }
.selected-module-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 12px; }
.selected-module { min-width: 0; padding: 11px; border-radius: 8px; background: var(--color-surface-muted); }
.selected-module strong { display: block; font-size: 12px; overflow-wrap: anywhere; }
.selected-module > span { display: block; margin-top: 5px; color: var(--color-text-muted); font-size: 11px; }
.term-list { display: grid; gap: 3px; margin-top: 8px; }
.term-list small { color: var(--color-text-secondary); font-size: 11px; overflow-wrap: anywhere; }
.muted { color: var(--color-text-muted); font-size: 12px; }
@media (max-width: 900px) { .selected-module-grid { grid-template-columns: 1fr; } }
</style>
