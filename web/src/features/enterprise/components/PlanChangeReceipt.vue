<script setup lang="ts">
import { computed } from 'vue'
import { currentUiLocale } from '@/i18n'
import { backendTermLabel } from '@/i18n/backend-terms'
import { UiButton } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import type { SubscriptionChangeReceiptDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{ receipt: SubscriptionChangeReceiptDTO; refreshing: boolean }>()
const emit = defineEmits<{ refresh: []; reset: [] }>()
const scheduled = computed(() => props.receipt.status === 'SCHEDULED')
const provisioning = computed(() => props.receipt.status === 'PROVISIONING')
const failed = computed(() => props.receipt.status === 'FAILED')
const applied = computed(() => props.receipt.status === 'APPLIED')
const renew = computed(() => props.receipt.action === 'RENEW')
const stopRenewal = computed(() => props.receipt.action === 'STOP_RENEWAL')
const resultTitle = computed(() => {
  const subject = renew.value ? '续订' : stopRenewal.value ? '停止续费' : '套餐变更'
  if (failed.value) return `${subject}未生效`
  if (scheduled.value) return `${subject}已预约`
  if (provisioning.value) return `${subject}处理中`
  if (applied.value) return stopRenewal.value ? '已停止续费' : `${subject}已生效`
  return backendTermLabel('receiptStatus', props.receipt.status)
})
const tone = computed(() => failed.value ? 'danger' : applied.value ? 'success' : scheduled.value || provisioning.value ? 'info' : 'neutral')
const expiry = computed(() => applied.value
  ? props.receipt.after?.periodEnd || props.receipt.entitlementExpiresAt
  : props.receipt.before?.periodEnd)
const recovery = computed(() => ({
  QUOTA_REVALIDATION_REQUIRED: '当前用量或额度已经变化。请先处理额度问题，再重新生成变更方案。',
  PREPARATION_REQUIREMENTS_CHANGED: '开通条件已经变化。请重新生成方案，系统会基于最新条件再次核对。',
  TARGET_NO_LONGER_ELIGIBLE: '目标套餐已不再可用。请重新选择当前可切换的套餐。',
  AUTHORITY_REVALIDATION_REQUIRED: '套餐或权益事实已经变化。请重新生成方案后再确认。',
  DEPENDENCY_REVALIDATION_REQUIRED: '相关配置已更新。请重新生成方案，系统会重新核对可用条件。',
}[props.receipt.failureCode] ?? '本次变更没有生效。请重新生成方案，系统会按当前套餐和权益事实重新核对。'))
function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
</script>

<template>
  <section data-plan-change-receipt class="receipt">
    <div :class="['result-hero', tone]">
      <span class="result-icon"><AppIcon :name="failed ? 'error' : applied ? 'success' : scheduled || provisioning ? 'clock' : 'help'" :size="24" /></span>
      <div class="flex-1">
        <h3>{{ resultTitle }}</h3>
        <p v-if="scheduled">本次变更已预约，尚未生效。请按计划时间查看处理结果。</p>
        <p v-else-if="provisioning">本次变更仍在处理中，尚未完成。</p>
        <p v-else-if="stopRenewal && applied">已停止后续续费，本操作未提前结束当前权益期。</p>
        <p v-else-if="applied">本次处理已生效，可返回套餐信息核对最新内容。</p>
        <p v-else-if="failed">本次变更未生效，请按下方说明重新处理。</p>
        <p v-else>暂不能确认本次处理结果，请重新检查。</p>
      </div>
      <strong>{{ backendTermLabel('receiptStatus', receipt.status) }}</strong>
    </div>
    <div class="receipt-facts">
      <div><span>操作编号</span><strong class="receipt-id">{{ receipt.changeId }}</strong></div>
      <div><span>处理状态</span><strong>{{ backendTermLabel('receiptStatus', receipt.status) }}</strong></div>
      <div><span>{{ applied ? '实际生效时间' : '计划生效时间' }}</span><strong>{{ formatDate(receipt.effectiveAt) }}</strong></div>
      <div v-if="renew || stopRenewal"><span>当前权益到期日</span><strong>{{ formatDate(expiry) }}</strong></div>
    </div>
    <p class="result-note">现有租户数据将被保留，本次操作不会删除资源。</p>
    <p v-if="stopRenewal && applied" class="result-note">停止续费不等于立即停止使用；到期后的权益按适用条款处理。</p>
    <div v-if="failed" class="recovery" data-plan-change-recovery>
      <div><h4>需要重新处理</h4><p>{{ recovery }}</p></div>
      <UiButton class="btn btn-primary" data-plan-change-recovery-restart @click="emit('reset')">重新生成变更方案</UiButton>
    </div>
    <div v-else class="receipt-actions">
      <UiButton v-if="!applied" class="btn" :disabled="refreshing" data-plan-change-receipt-refresh @click="emit('refresh')">{{ refreshing ? '正在刷新…' : '刷新处理结果' }}</UiButton>
      <UiButton v-if="applied" class="btn" @click="emit('reset')">发起其他变更</UiButton>
    </div>
  </section>
</template>

<style scoped>
.receipt { display: grid; gap: 14px; }
.result-hero { display: flex; align-items: center; gap: 14px; padding: 18px; border-radius: var(--radius-md); background: var(--color-surface-soft); }
.result-hero.info { background: var(--color-primary-soft); }
.result-hero.success { background: var(--color-success-soft); }
.result-hero.danger { background: var(--color-danger-soft); }
.result-icon { width: 42px; height: 42px; flex-shrink: 0; display: grid; place-items: center; border-radius: 50%; background: var(--color-surface); color: var(--color-text-muted); }
.info .result-icon { color: var(--color-primary); }
.success .result-icon { color: var(--color-success); }
.danger .result-icon { color: var(--color-danger); }
.result-hero h3 { margin: 0; font-size: var(--text-lg); line-height: 1.5; }
.result-hero p { margin-top: 5px; color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.receipt-facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.receipt-facts > div { min-width: 0; padding: 13px; border-radius: var(--radius-sm); background: var(--color-surface-soft); }
.receipt-facts span { display: block; margin-bottom: 5px; color: var(--color-text-muted); font-size: var(--text-xs); }
.receipt-facts strong { display: block; overflow-wrap: anywhere; font-size: var(--text-sm); }
.receipt-id { font-variant-numeric: tabular-nums; }
.result-note { padding: 13px 14px; border-radius: var(--radius-sm); background: var(--color-primary-soft); color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.6; }
.recovery { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-warning-soft); }
.recovery h4 { font-size: var(--text-base); line-height: 1.5; }
.recovery p { margin-top: 5px; max-width: 66ch; color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.receipt-actions { display: flex; justify-content: flex-end; gap: 10px; }
@media (max-width: 900px) { .receipt-facts { grid-template-columns: 1fr 1fr; } }
@media (max-width: 767px) { .receipt-facts { grid-template-columns: 1fr; } .recovery, .result-hero { align-items: flex-start; flex-direction: column; } }
</style>
