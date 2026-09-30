<script setup lang="ts">
import { computed } from 'vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import { UiButton } from '@/ui/base'
import type { SubscriptionChangeReceiptDTO } from '@/services/commercial/platformCommercial'
const props = defineProps<{ receipt: SubscriptionChangeReceiptDTO, refreshing: boolean }>()
const emit = defineEmits<{ refresh: [], reset: [] }>()
const refreshing = computed(() => ['SCHEDULED','PROVISIONING'].includes(props.receipt.status))
const failed = computed(() => props.receipt.status === 'FAILED')
const recovery = computed(() => ({
  QUOTA_REVALIDATION_REQUIRED: '当前用量或额度已经变化。请先处理额度问题，再重新生成变更方案。',
  PREPARATION_REQUIREMENTS_CHANGED: '开通条件已经变化。请重新生成方案，系统会基于最新条件再次核对。',
  TARGET_NO_LONGER_ELIGIBLE: '目标套餐已不再可用。请重新选择当前可切换的套餐。',
  AUTHORITY_REVALIDATION_REQUIRED: '套餐或权益事实已经变化。请重新生成方案后再确认。',
  DEPENDENCY_REVALIDATION_REQUIRED: '相关配置已更新。请重新生成方案，系统会重新核对可用条件。',
}[props.receipt.failureCode] ?? '本次变更没有生效。请重新生成方案，系统会按当前套餐和权益事实重新核对。'))
</script>
<template>
  <section data-plan-change-receipt class="receipt">
    <h3>{{ backendTermLabel('receiptStatus', receipt.status) }}</h3>
    <p class="receipt-id">{{ receipt.changeId }}</p>
    <p>现有租户数据将被保留，本次操作不会删除资源。</p>
    <div v-if="failed" class="recovery" data-plan-change-recovery>
      <h4>需要重新处理</h4>
      <p>{{ recovery }}</p>
      <UiButton class="btn primary" data-plan-change-recovery-restart @click="emit('reset')">重新生成变更方案</UiButton>
    </div>
    <template v-else>
      <UiButton v-if="refreshing" class="btn" :disabled="props.refreshing" data-plan-change-receipt-refresh @click="emit('refresh')">刷新处理结果</UiButton>
      <UiButton class="btn" @click="emit('reset')">发起其他变更</UiButton>
    </template>
  </section>
</template>
<style scoped>
.receipt { display: grid; gap: 10px; }
.receipt h3 { margin: 0; font-size: 18px; line-height: 1.2; }
.receipt p { margin: 0; max-width: 66ch; color: var(--color-text-secondary); font-size: 13px; line-height: 1.55; text-wrap: pretty; }
.receipt-id { overflow-wrap: break-word; font-variant-numeric: tabular-nums; }
.recovery { display: grid; gap: 8px; padding: 14px; border: 1px solid var(--color-warning-border, var(--color-border)); border-radius: 10px; background: var(--color-warning-soft, var(--color-surface-muted)); }
.recovery h4 { margin: 0; font-size: 14px; line-height: 1.3; }
</style>
