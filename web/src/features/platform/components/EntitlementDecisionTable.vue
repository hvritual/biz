<script setup lang="ts">
import StatusBadge from '@/ui/common/StatusBadge.vue'
import type { EntitlementDecisionDTO } from '@/services/commercial/platformCommercial'

defineProps<{ decisions: EntitlementDecisionDTO[] }>()

function targetLabel(item: EntitlementDecisionDTO) {
  const labels: Record<string, string> = {
    'device.lifecycle': '设备生命周期',
    'customer.view': '客户查看',
    'customer.count': '客户额度',
    'member.count': '成员额度',
  }
  const actionLabels: Record<string, string> = { read: '查看', write: '修改', export: '导出' }
  const target = labels[item.key] ?? (item.key ? '配置项目' : '整个模块')
  return item.fieldAction ? `${target} · ${actionLabels[item.fieldAction] ?? '业务操作'}` : target
}

function kindLabel(value: string) {
  if (value === 'capability') return '功能能力'
  if (value === 'quota') return '使用额度'
  if (value === 'field') return '数据字段'
  if (value === 'module') return '模块权益'
  return '权益结果'
}

function sourceKindLabel(value: string) {
  if (value === 'plan') return '套餐权益'
  if (value === 'override') return '专项权益'
  if (value === 'addon') return '增购权益'
  if (value === 'subscription') return '订阅权益'
  return '权益来源'
}

function sourceEffectLabel(value: string) {
  if (/DENY/i.test(value)) return '拒绝'
  if (/GRANT|ALLOW/i.test(value)) return '允许'
  if (/MASK/i.test(value)) return '脱敏'
  return '已应用'
}

function sourceStateLabel(value: string) {
  if (/ACTIVE/i.test(value)) return '生效中'
  if (/REVOKED/i.test(value)) return '已撤销'
  if (/EXPIRED/i.test(value)) return '已过期'
  if (/PENDING|SCHEDULED/i.test(value)) return '待生效'
  return '当前有效'
}

function dispositionLabel(value: string) {
  if (/ignored|skipped/i.test(value)) return '未应用'
  if (/applied|effective|used/i.test(value)) return '已应用'
  return '已纳入判断'
}

function decisionLabel(item: EntitlementDecisionDTO) {
  if (!item.allowed) return '拒绝'
  if (item.masked) return '允许 · 脱敏'
  return '允许'
}

function decisionTone(item: EntitlementDecisionDTO): 'success' | 'warning' | 'neutral' {
  if (!item.allowed) return 'warning'
  if (item.masked) return 'neutral'
  return 'success'
}

function limitLabel(item: EntitlementDecisionDTO) {
  if (!item.limit) return '—'
  return item.limit.unlimited ? '不限' : String(item.limit.value ?? 0)
}
</script>

<template>
  <section class="card decision-card">
    <div class="section-header">
      <div><h2>权益结果与来源</h2><p>查看当前权益结果、来源与调整原因，便于确认租户实际可用能力。</p></div>
      <span class="count">{{ decisions.length }} 项</span>
    </div>

    <div v-if="!decisions.length" class="empty">当前解析结果没有权益决策。</div>
    <div v-else class="decision-list">
      <article v-for="(item, index) in decisions" :key="`${item.kind}-${item.moduleCode}-${item.key}-${item.fieldAction}-${index}`" class="decision-row">
        <div class="decision-main">
          <div class="decision-title">
            <strong>{{ kindLabel(item.kind) }}</strong>
            <StatusBadge :text="decisionLabel(item)" :tone="decisionTone(item)" />
          </div>
          <p class="target">{{ targetLabel(item) }}</p>
          <div class="decision-meta"><span>判断说明：{{ item.reason || '—' }}</span><span>额度：{{ limitLabel(item) }}</span></div>
        </div>

        <div class="sources">
          <strong>来源链</strong>
          <p v-if="!item.sources?.length" class="muted">无来源解释</p>
          <div v-for="source in item.sources ?? []" :key="`${source.id}-${source.sourceKind}-${source.effect}`" class="source-row">
            <div><b>{{ sourceKindLabel(source.sourceKind) }}</b><span>{{ sourceEffectLabel(source.effect) }}</span><span>{{ sourceStateLabel(source.state) }}</span></div>
            <p>{{ dispositionLabel(source.disposition) }}<template v-if="source.reason"> · {{ source.reason }}</template></p>
            <small>记录 {{ source.id || '—' }}<template v-if="source.actorId"> · 操作人 {{ source.actorId }}</template></small>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.decision-card { overflow: hidden; }
.section-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.section-header h2 { margin: 0; font-size: 16px; }
.section-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.count { flex: 0 0 auto; font-size: 12px; color: var(--color-text-muted); }
.empty { padding: 28px 20px; color: var(--color-text-muted); }
.decision-list { display: grid; }
.decision-row { display: grid; grid-template-columns: minmax(220px, .8fr) minmax(300px, 1.2fr); gap: 18px; padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.decision-row:last-child { border-bottom: 0; }
.decision-title { display: flex; align-items: center; gap: 10px; }
.target { margin: 6px 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.decision-meta { display: flex; flex-wrap: wrap; gap: 8px 14px; color: var(--color-text-muted); font-size: 11px; }
.sources { display: grid; gap: 8px; }
.sources > strong { font-size: 12px; color: var(--color-text-secondary); }
.source-row { padding: 10px 12px; border-radius: 9px; background: var(--color-surface-subtle); }
.source-row div { display: flex; flex-wrap: wrap; gap: 8px 12px; font-size: 12px; }
.source-row p, .source-row small { margin: 4px 0 0; color: var(--color-text-muted); font-size: 11px; }
.muted { margin: 0; color: var(--color-text-muted); font-size: 12px; }
@media (max-width: 820px) { .decision-row { grid-template-columns: 1fr; } }
</style>
