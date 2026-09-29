<script setup lang="ts">
import { computed } from 'vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import { UiButton } from '@/ui/base'
import type { SubscriptionChangePreviewDTO } from '@/services/commercial/platformCommercial'
import { currentUiLocale } from '@/i18n'

const props = defineProps<{ open: boolean, preview: SubscriptionChangePreviewDTO, pending: boolean }>()
const emit = defineEmits<{ close: [], confirm: [] }>()
const price = computed(() => {
  const terms = props.preview.target?.terms
  if (!terms?.priceRef) return '免费开通'
  const amount = Number(terms.amountMinor)
  return Number.isSafeInteger(amount) && amount > 0 && /^[A-Z]{3}$/.test(terms.currency) ? new Intl.NumberFormat(currentUiLocale(), { style: 'currency', currency: terms.currency }).format(amount / 100) : '价格待确认'
})
function date(value?: string) { const result = new Date(value ?? ''); return !value ? '—' : Number.isNaN(result.getTime()) ? value : new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(result) }
</script>

<template>
  <UiDialog :open="open" title="确认套餐变更" @close="emit('close')">
    <div class="summary" data-plan-change-confirm-dialog><p>请核对本次套餐、价格与生效时间；确认后将按预览方案处理。</p><dl><div><dt>目标套餐</dt><dd>{{ preview.target?.name || '目标套餐' }}</dd></div><div><dt>价格</dt><dd>{{ price }}</dd></div><div><dt>权益生效</dt><dd>{{ date(preview.effectiveAt) }}</dd></div></dl></div>
    <template #footer><UiButton class="btn" :disabled="pending" @click="emit('close')">返回修改</UiButton><UiButton class="btn primary" :disabled="pending" data-plan-change-confirm-dialog-submit @click="emit('confirm')">{{ pending ? '正在确认…' : '确认开通' }}</UiButton></template>
  </UiDialog>
</template>

<style scoped>
.summary p { margin: 0; color: var(--color-text-secondary); font-size: 13px; line-height: 1.6; }.summary dl { display: grid; gap: 10px; margin: 16px 0 0; }.summary dl div { display: grid; grid-template-columns: 100px 1fr; gap: 12px; padding: 10px; border-radius: 8px; background: var(--color-surface-muted); }.summary dt { color: var(--color-text-muted); font-size: 12px; }.summary dd { margin: 0; color: var(--color-text-primary); font-size: 13px; overflow-wrap: anywhere; }
</style>
