<script setup lang="ts">
import { backendTermLabel } from '@/i18n/backend-terms'
import type { SubscriptionChangePreviewDTO } from '@/services/commercial/platformCommercial'
defineProps<{ preview: SubscriptionChangePreviewDTO }>()
</script>
<template>
  <div class="impact-grid"><div class="impact-card"><h4>额度影响</h4><div v-if="preview.quotaImpacts?.length" class="impact-list"><div v-for="(item,index) in preview.quotaImpacts" :key="`${item.moduleCode}:${item.key}`"><span>{{ backendTermLabel('entitlementKey',item.key) }} · {{ index+1 }}</span><strong>{{ item.usageKnown?`已用 ${item.used}`:'用量未知' }}</strong><small>{{ item.overLimit?'超过目标额度':item.policy||'通过当前校验' }}</small></div></div><p v-else class="muted">本次变更没有额度差异。</p></div><div class="impact-card"><h4>准备与依赖</h4><p v-if="preview.provisioningRequirements?.length">需要完成 {{ preview.provisioningRequirements.length }} 项准备工作后再生效。</p><p v-else>无需额外准备，可按当前方案生效。</p><p v-if="preview.dependencies?.length" class="muted">涉及 {{ preview.dependencies.length }} 个模块依赖。</p></div></div>
</template>
<style scoped>
.impact-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:14px}.impact-card{padding:16px;border:1px solid var(--color-border);border-radius:10px}.impact-card h4{margin-bottom:10px;font-size:13px}.impact-card p{color:var(--color-text-secondary);font-size:12px;line-height:1.6}.impact-list{display:grid;gap:8px}.impact-list>div{display:grid;grid-template-columns:1fr auto;gap:2px 12px;padding-bottom:8px;border-bottom:1px solid var(--color-border)}.impact-list small{grid-column:1/-1;color:var(--color-text-muted)}.muted{color:var(--color-text-muted);font-size:12px}@media(max-width:900px){.impact-grid{grid-template-columns:1fr}}
</style>
