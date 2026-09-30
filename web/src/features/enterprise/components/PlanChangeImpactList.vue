<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import type { SubscriptionChangeImpact } from '@/services/commercial/platformCommercial'

const props = defineProps<{ impacts: SubscriptionChangeImpact[], fallbackMessages: string[] }>()
const structured = computed(() => props.impacts ?? [])
const title = (impact: SubscriptionChangeImpact) => ({ DATA_PRESERVED: '现有数据保留', ENTITLEMENT_PROJECTION: '权益将按方案重新核对', NO_NEW_ENFORCEMENT: '本次不新增资源限制', SELF_SERVICE_CONFIRMATION: '确认后才会提交变更', EXTERNAL_COMMERCIAL_APPROVAL: '需要完成商业或支付审批', SCHEDULED_EFFECTIVE_TIME: '将在预约时间生效', QUOTA_REVALIDATION: '需要重新核对额度', AUTO_RENEWAL_DISABLED: '自动续费将关闭', EXTERNAL_PREPARATION: '需要完成开通准备' }[impact.code] ?? '套餐变更影响')
const description = (impact: SubscriptionChangeImpact) => ({ DATA_PRESERVED: '已有业务数据不会因本次套餐变更被删除。', ENTITLEMENT_PROJECTION: '套餐、专项授权和安全限制将按当前事实重新计算。', NO_NEW_ENFORCEMENT: '本次预览不会把展示用量当作新的资源执行限制。', SELF_SERVICE_CONFIRMATION: '当前仅为预览；在完成确认前，套餐与权益不会改变。', EXTERNAL_COMMERCIAL_APPROVAL: '该套餐包含价格事实，必须先取得外部商业或支付审批。', SCHEDULED_EFFECTIVE_TIME: '当前权益保持有效，系统会在预约时间再次核对后执行。', QUOTA_REVALIDATION: '当前用量未知或超出目标，需要先满足额度要求。', AUTO_RENEWAL_DISABLED: '当前权益将持续至到期日，之后不会自动续订。', EXTERNAL_PREPARATION: '系统已保留处理意图，实际权益要等待准备完成后才会生效。' }[impact.code] ?? '请根据当前套餐变更结果确认后续操作。')
const tone = (impact: SubscriptionChangeImpact) => impact.severity === 'DANGER' ? 'danger' : impact.severity === 'WARNING' ? 'warning' : 'neutral'
</script>
<template>
  <div class="impact-card impacts"><h4>变更影响</h4><div v-if="structured.length" class="structured-impact-list"><article v-for="impact in structured" :key="impact.code" :class="['structured-impact',{blocking:impact.blocking}]"><StatusBadge :text="impact.blocking?'需要处理':'已说明'" :tone="tone(impact)" :dot="false"/><div><strong>{{ title(impact) }}</strong><p>{{ description(impact) }}</p><small v-if="impact.usageKnown">当前用量：{{ impact.currentUsage }}</small><small v-if="impact.actionRequired" class="impact-action">下一步：{{ title(impact) }}</small></div></article></div><p v-else-if="fallbackMessages.length" class="impact-summary">现有租户数据将被保留，本次操作不会删除资源；确认时将按当前套餐变更事实重新核对。</p></div>
</template>
<style scoped>
.impacts{margin-top:14px}.impacts ul{margin:0;padding-left:18px}.structured-impact-list{display:grid;gap:8px}.structured-impact{display:grid;grid-template-columns:auto minmax(0,1fr);gap:10px;align-items:start;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-surface-soft)}.structured-impact.blocking{border-color:var(--color-warning)}.structured-impact strong,.structured-impact p,.structured-impact small{display:block}.structured-impact p{margin-top:3px;color:var(--color-text-secondary);font-size:var(--text-xs);line-height:1.5}.structured-impact small{margin-top:4px;color:var(--color-text-muted);font-size:var(--text-xs)}.structured-impact .impact-action{color:var(--color-text-secondary)}
</style>
