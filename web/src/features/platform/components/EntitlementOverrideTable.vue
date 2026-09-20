<script setup lang="ts">
import { UiButton } from '@/ui/base'

import StatusBadge from '@/ui/common/StatusBadge.vue'
import type { EntitlementOverrideDTO } from '@/services/commercial/platformCommercial'

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
  if (source.revokedAt) return { text: '已撤销', tone: 'neutral' as const }
  const now = Date.now()
  const starts = source.effectiveAt ? new Date(source.effectiveAt).valueOf() : Number.NEGATIVE_INFINITY
  const ends = source.expiresAt ? new Date(source.expiresAt).valueOf() : Number.POSITIVE_INFINITY
  if (Number.isFinite(starts) && starts > now) return { text: '待生效', tone: 'warning' as const }
  if (Number.isFinite(ends) && ends <= now) return { text: '已到期', tone: 'neutral' as const }
  return { text: '生效中', tone: 'success' as const }
}

function targetLabel(source: EntitlementOverrideDTO) {
  const labels: Record<string, string> = {
    'device.lifecycle': '设备生命周期',
    'customer.view': '客户查看',
    'customer.count': '客户额度',
    'member.count': '成员额度',
  }
  const actionLabels: Record<string, string> = { read: '查看', write: '修改', export: '导出' }
  const target = labels[source.key] ?? (source.key ? '配置项目' : '整个模块')
  return source.fieldAction ? `${target} · ${actionLabels[source.fieldAction] ?? '业务操作'}` : target
}

function sourceKindLabel(value: string) {
  if (value === 'override') return '专项授权'
  if (value === 'plan') return '套餐权益'
  if (value === 'addon') return '增购权益'
  return '专项权益'
}

function effectLabel(effect: string) {
  const labels: Record<string, string> = {
    ENTITLEMENT_EFFECT_GRANT: '授权',
    ENTITLEMENT_EFFECT_DENY: '拒绝',
    ENTITLEMENT_EFFECT_QUOTA_ADD: '额度追加',
    ENTITLEMENT_EFFECT_QUOTA_REPLACE: '额度替换',
    ENTITLEMENT_EFFECT_SAFETY_DENY: '安全拒绝',
    ENTITLEMENT_EFFECT_SAFETY_MASK: '安全脱敏',
  }
  return labels[effect] || effect || '—'
}

function limitLabel(source: EntitlementOverrideDTO) {
  if (!source.limit) return '—'
  return source.limit.unlimited ? '不限' : String(source.limit.value ?? 0)
}
</script>

<template>
  <section class="card source-card">
    <div class="section-header">
      <div><h2>专项权益</h2><p>当前版本 {{ sourceVersion }}；撤销后不再生效，历史记录仍会保留。</p></div>
      <UiButton class="btn primary" type="button" :disabled="pending" @click="emit('create')">新增专项权益</UiButton>
    </div>

    <div v-if="!sources.length" class="empty">当前租户没有专项权益。</div>
    <div v-else class="table-scroll">
      <table class="data-table">
        <thead><tr><th>权益类型</th><th>授权项目</th><th>授权结果</th><th>额度</th><th>状态</th><th>生效时间</th><th>原因 / 操作人</th><th></th></tr></thead>
        <tbody>
          <tr v-for="source in sources" :key="source.id">
            <td><strong>{{ sourceKindLabel(source.sourceKind) }}</strong><small>记录 {{ source.id }}</small></td>
            <td>{{ targetLabel(source) }}</td>
            <td>{{ effectLabel(source.effect) }}</td>
            <td>{{ limitLabel(source) }}</td>
            <td><StatusBadge :text="sourceState(source).text" :tone="sourceState(source).tone" /></td>
            <td><small>生效：{{ source.effectiveAt || '创建时' }}</small><small>到期：{{ source.expiresAt || '无到期时间' }}</small></td>
            <td><span>{{ source.reason || '—' }}</span><small>{{ source.actorId ? `账号 ${source.actorId}` : '—' }}</small></td>
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
