<script setup lang="ts">
import { computed } from 'vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import { UiButton } from '@/ui/base'
import { backendTermLabel } from '@/i18n/backend-terms'
import { currentUiLocale } from '@/i18n'
import type { SubscriptionChangePreviewDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{ open: boolean; preview: SubscriptionChangePreviewDTO; pending: boolean }>()
const emit = defineEmits<{ close: []; confirm: [] }>()
const renew = computed(() => props.preview.action === 'RENEW')
const stopRenewal = computed(() => props.preview.action === 'STOP_RENEWAL')
const title = computed(() => renew.value ? '确认续订当前套餐' : stopRenewal.value ? '确认停止自动续费' : '确认套餐变更')
const submitLabel = computed(() => renew.value ? '确认续订' : stopRenewal.value ? '确认停止' : '确认开通')
const targetLabel = computed(() => renew.value || stopRenewal.value
  ? backendTermLabel('plan', props.preview.before?.planCode)
  : props.preview.target?.name || backendTermLabel('plan', props.preview.target?.planCode))
function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
</script>

<template>
  <UiDialog :open="open" :title="title" @close="emit('close')">
    <div class="confirm-stack" data-plan-change-confirm-dialog>
      <p>请核对本次操作及生效时间；确认后将按本次方案处理。</p>
      <div class="confirm-target"><span>{{ renew ? '续订对象' : stopRenewal ? '停止续费对象' : '目标套餐' }}</span><strong>{{ targetLabel }}</strong></div>
      <div v-if="renew || stopRenewal" class="confirm-fact"><span>当前权益到期日</span><strong>{{ formatDate(preview.before?.periodEnd) }}</strong></div>
      <div v-if="renew" class="confirm-fact"><span>续订后权益到期日</span><strong>{{ formatDate(preview.entitlementExpiresAt) }}</strong></div>
      <div class="confirm-fact"><span>本次生效时间</span><strong>{{ formatDate(preview.effectiveAt) }}</strong></div>
      <p v-if="stopRenewal" class="notice-box warning">停止续费不会缩短当前权益期，也不会删除资源或立即停用模块。</p>
      <p v-else-if="preview.action === 'SWITCH' && preview.pricingBasis === 'NO_PRICE_REFERENCE'">免费开通。本次确认不会创建支付订单。</p>
      <p v-else>{{ preview.target?.terms?.priceRef ? '费用按销售方案确认' : '本次确认不代表已经付款。' }}</p>
      <p>提交受理不等于处理完成，请继续查看处理结果。</p>
    </div>
    <template #footer>
      <UiButton class="btn" :disabled="pending" @click="emit('close')">返回修改</UiButton>
      <UiButton class="btn btn-primary" :disabled="pending" data-plan-change-confirm-dialog-submit @click="emit('confirm')">{{ pending ? '正在确认…' : submitLabel }}</UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.confirm-stack { display: grid; gap: 16px; color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.confirm-target { display: grid; gap: 5px; padding: 14px; border-radius: var(--radius-md); background: var(--color-surface-soft); }
.confirm-target span { color: var(--color-text-muted); font-size: var(--text-xs); }
.confirm-target strong { color: var(--color-text); font-size: var(--text-base); }
.confirm-fact { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; padding-bottom: 10px; border-bottom: 1px solid var(--color-border); }
.confirm-fact strong { color: var(--color-text); font-variant-numeric: tabular-nums; }
</style>
