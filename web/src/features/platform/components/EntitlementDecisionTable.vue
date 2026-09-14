<script setup lang="ts">
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { EntitlementDecisionDTO } from '@/services/commercial/platformCommercial'

defineProps<{ decisions: EntitlementDecisionDTO[] }>()

function targetLabel(item: EntitlementDecisionDTO) {
  const parts = [item.moduleCode, item.key, item.fieldAction].filter(Boolean)
  return parts.join(' / ') || '—'
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
  return item.limit.unlimited ? 'unlimited' : String(item.limit.value ?? 0)
}
</script>

<template>
  <section class="card decision-card">
    <div class="section-header">
      <div><h2>权益决策与来源解释</h2><p>平台解释保留来源 actor / reason；决策由服务端 resolver 产生，前端不自行合并来源。</p></div>
      <span class="count">{{ decisions.length }} 项</span>
    </div>

    <div v-if="!decisions.length" class="empty">当前解析结果没有权益决策。</div>
    <div v-else class="decision-list">
      <article v-for="(item, index) in decisions" :key="`${item.kind}-${item.moduleCode}-${item.key}-${item.fieldAction}-${index}`" class="decision-row">
        <div class="decision-main">
          <div class="decision-title">
            <strong>{{ item.kind || 'decision' }}</strong>
            <StatusBadge :text="decisionLabel(item)" :tone="decisionTone(item)" />
          </div>
          <p class="target">{{ targetLabel(item) }}</p>
          <div class="decision-meta"><span>reason: {{ item.reason || '—' }}</span><span>limit: {{ limitLabel(item) }}</span></div>
        </div>

        <div class="sources">
          <strong>来源链</strong>
          <p v-if="!item.sources?.length" class="muted">无来源解释</p>
          <div v-for="source in item.sources ?? []" :key="`${source.id}-${source.sourceKind}-${source.effect}`" class="source-row">
            <div><b>{{ source.sourceKind || 'unknown' }}</b><span>{{ source.effect || '—' }}</span><span>{{ source.state || '—' }}</span></div>
            <p>{{ source.disposition || '—' }}<template v-if="source.reason"> · {{ source.reason }}</template></p>
            <small>source {{ source.id || '—' }}<template v-if="source.actorId"> · actor {{ source.actorId }}</template></small>
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
