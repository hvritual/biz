<script setup lang="ts">
import { computed } from 'vue'
import { currentUiLocale } from '@/i18n'
import { backendTermLabel } from '@/i18n/backend-terms'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { UiButton } from '@/ui/base'
import type { SubscriptionChangeReceiptDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{ receipt: SubscriptionChangeReceiptDTO, refreshing: boolean }>()
const emit = defineEmits<{ refresh: [], reset: [] }>()
const failed = computed(() => props.receipt.status === 'FAILED')
const tone = computed(() => props.receipt.status === 'APPLIED' ? 'success' : failed.value || ['SCHEDULED', 'PROVISIONING'].includes(props.receipt.status) ? 'warning' : 'neutral')
const canRefresh = computed(() => ['SCHEDULED', 'PROVISIONING'].includes(props.receipt.status))
function label(kind: 'receiptStatus' | 'effectiveMode', value?: string) { return backendTermLabel(kind, value) }
function time(value?: string) { const date = new Date(value ?? ''); return !value ? '—' : Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(date) }
</script>

<template>
  <div data-plan-change-receipt>
    <div class="hero"><span class="icon"><AppIcon name="check" :size="28" /></span><div><span class="label">套餐变更结果</span><h3>{{ label('receiptStatus', receipt.status) }}</h3><p>确认时间 {{ time(receipt.confirmedAt) }} · 生效时间 {{ time(receipt.effectiveAt) }}</p></div><StatusBadge :text="label('receiptStatus', receipt.status)" :tone="tone" :dot="false" /></div>
    <div class="facts"><div><span>变更编号</span><strong>{{ receipt.changeId }}</strong></div><div><span>生效方式</span><strong>{{ label('effectiveMode', receipt.mode) }}</strong></div><div><span>后续处理</span><strong>{{ receipt.provisioningTaskId ? '需要进一步处理' : '无需额外处理' }}</strong></div></div>
    <div class="note"><strong v-if="receipt.status === 'APPLIED'">套餐变更已生效。</strong><strong v-else-if="receipt.status === 'SCHEDULED'">套餐变更已预约。</strong><strong v-else-if="receipt.status === 'PROVISIONING'">套餐变更正在外部准备。</strong><strong v-else-if="failed">变更未生效，当前权益保持不变。</strong><span v-if="failed">请先核对当前套餐和权益状态；确认无误后重新生成预览。系统不会自动重试或把失败回执视为成功。</span><span v-else>处理未完成时会自动更新状态；也可手动刷新查看最新结果。</span></div>
    <div class="actions"><UiButton v-if="canRefresh" class="btn" :disabled="refreshing" data-plan-change-receipt-refresh @click="emit('refresh')">{{ refreshing ? '正在刷新…' : '刷新处理结果' }}</UiButton><UiButton class="btn" @click="emit('reset')">{{ failed ? '核对后重新生成方案' : '发起其他变更' }}</UiButton></div>
  </div>
</template>

<style scoped>
.hero { display: flex; align-items: center; gap: 16px; padding: 18px; border-radius: 12px; background: var(--color-primary-soft); }.hero > div { flex: 1; min-width: 0; }.hero h3 { margin-top: 3px; font-size: 20px; }.hero p, .note, .facts span { color: var(--color-text-secondary); font-size: 12px; }.label { color: var(--color-text-muted); font-size: 12px; font-weight: 600; }.icon { width: 48px; height: 48px; display: grid; place-items: center; border-radius: 50%; background: var(--color-primary); color: var(--color-on-primary); }.facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }.facts div, .note { padding: 13px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-muted); }.facts span, .facts strong { display: block; }.facts strong { margin-top: 5px; font-size: 12px; overflow-wrap: anywhere; }.note { display: grid; gap: 3px; margin-top: 14px; line-height: 1.6; }.actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 20px; } @media (max-width: 560px) { .facts { grid-template-columns: 1fr; }.actions { flex-direction: column; }.actions .btn { width: 100%; } }
</style>
