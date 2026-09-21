<script setup lang="ts">
import { UiButton } from '@/ui/base'

import StatusBadge from '@/ui/common/StatusBadge.vue'
import type { EntitlementOverrideDTO } from '@/services/commercial/platformCommercial'
import { backendBusinessText, backendTermLabel } from '@/i18n/backend-terms'

defineProps<{
  sources: EntitlementOverrideDTO[]
  sourceVersion: string | number
  pending: boolean
}>()

const emit = defineEmits<{
  create: []
  revoke: [source: EntitlementOverrideDTO]
}>()

function sourceState(source: EntitlementOverrideDTO) {
  if (source.revokedAt) return { text: backendTermLabel('sourceState', 'REVOKED'), tone: 'neutral' as const }
  const now = Date.now()
  const starts = source.effectiveAt ? new Date(source.effectiveAt).valueOf() : Number.NEGATIVE_INFINITY
  const ends = source.expiresAt ? new Date(source.expiresAt).valueOf() : Number.POSITIVE_INFINITY
  if (Number.isFinite(starts) && starts > now) return { text: backendTermLabel('sourceState', 'PENDING'), tone: 'warning' as const }
  if (Number.isFinite(ends) && ends <= now) return { text: backendTermLabel('sourceState', 'EXPIRED'), tone: 'neutral' as const }
  return { text: backendTermLabel('sourceState', 'ACTIVE'), tone: 'success' as const }
}

function targetLabel(source: EntitlementOverrideDTO) {
  const target = source.key
    ? backendTermLabel('entitlementKey', source.key)
    : backendTermLabel('entitlementTarget', 'ENTITLEMENT_TARGET_MODULE')
  return source.fieldAction ? `${target} · ${backendTermLabel('fieldAction', source.fieldAction)}` : target
}


function limitLabel(source: EntitlementOverrideDTO) {
  if (!source.limit) return '—'
  return source.limit.unlimited ? '不限' : String(source.limit.value ?? 0)
}
</script>

<template>
  <section class="card source-card">
    <div class="section-header">
      <div><h2>专项权益</h2><p>撤销后不再生效，历史记录仍会保留。</p></div>
      <UiButton class="btn primary" type="button" :disabled="pending" @click="emit('create')">新增专项权益</UiButton>
    </div>

    <div v-if="!sources.length" class="empty">当前租户没有专项权益。</div>
    <div v-else class="table-scroll">
      <table class="data-table">
        <thead><tr><th>权益类型</th><th>授权项目</th><th>授权结果</th><th>额度</th><th>状态</th><th>生效时间</th><th>原因 / 操作人</th><th></th></tr></thead>
        <tbody>
          <tr v-for="source in sources" :key="source.id">
            <td><strong>{{ backendTermLabel('sourceKind', source.sourceKind) }}</strong><small>专项权益记录已保留</small></td>
            <td>{{ targetLabel(source) }}</td>
            <td>{{ backendTermLabel('entitlementEffect', source.effect) }}</td>
            <td>{{ limitLabel(source) }}</td>
            <td><StatusBadge :text="sourceState(source).text" :tone="sourceState(source).tone" /></td>
            <td><small>生效：{{ source.effectiveAt || '创建时' }}</small><small>到期：{{ source.expiresAt || '无到期时间' }}</small></td>
            <td><span>{{ source.reason ? backendBusinessText(source.reason) : '—' }}</span><small>{{ source.actorId ? '操作人已记录' : '—' }}</small></td>
            <td><UiButton v-if="!source.revokedAt" class="btn small danger" type="button" :disabled="pending" @click="emit('revoke', source)">撤销</UiButton></td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.source-card { overflow: hidden; }
.section-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.section-header h2 { margin: 0; font-size: 16px; }
.section-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.empty { padding: 28px 20px; color: var(--color-text-muted); }
td small { display: block; margin-top: 3px; color: var(--color-text-muted); font-size: 11px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.btn.small { padding: 6px 9px; font-size: 12px; }
.btn.danger { color: var(--color-danger, var(--color-fixed-fdb6b83b)); }
</style>
