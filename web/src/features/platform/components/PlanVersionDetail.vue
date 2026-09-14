<script setup lang="ts">
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { ModuleDTO, PlanTerms, PlanVersionDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{
  version: PlanVersionDTO
  modules: ModuleDTO[]
  pending: boolean
}>()

const emit = defineEmits<{
  edit: []
  publish: []
  retire: []
  clone: []
  eligibility: []
}>()

function statusLabel(state: string) {
  const labels: Record<string, string> = { DRAFT: '草稿', PUBLISHED: '已发布', RETIRED: '已停售' }
  return labels[state] || state || '未知'
}

function statusTone(state: string): 'success' | 'warning' | 'neutral' {
  if (state === 'PUBLISHED') return 'success'
  if (state === 'DRAFT') return 'warning'
  return 'neutral'
}

function formatTime(value: string) {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? value : parsed.toLocaleString('zh-CN', { hour12: false })
}

function validityLabel(terms?: PlanTerms) {
  if (!terms) return '—'
  if (terms.validityMode === 'fixed_days') return `${terms.validityDays} 天`
  return terms.validityMode === 'unlimited' ? '长期有效' : terms.validityMode || '—'
}

function moduleName(code: string) {
  return props.modules.find((item) => item.moduleCode === code)?.name || code
}
</script>

<template>
  <section class="card plan-detail">
    <header class="detail-header">
      <div>
        <div class="title-row">
          <h2>{{ version.name || version.planCode }}</h2>
          <StatusBadge :text="statusLabel(version.state)" :tone="statusTone(version.state)" />
        </div>
        <p>{{ version.planCode }} · v{{ version.version }} · plan revision {{ version.planRevision }}</p>
      </div>
      <div class="actions">
        <button v-if="version.state === 'DRAFT'" class="btn" type="button" @click="emit('edit')">编辑草稿</button>
        <button v-if="version.state === 'DRAFT'" class="btn primary" type="button" :disabled="pending" @click="emit('publish')">发布</button>
        <button v-if="version.state === 'PUBLISHED'" class="btn" type="button" :disabled="pending" @click="emit('retire')">停售</button>
        <button v-if="version.state !== 'DRAFT'" class="btn" type="button" :disabled="pending" @click="emit('clone')">创建新版本</button>
        <button class="btn" type="button" @click="emit('eligibility')">资格预检</button>
      </div>
    </header>

    <dl class="facts">
      <div><dt>有效期</dt><dd>{{ validityLabel(version.terms) }}</dd></div>
      <div><dt>价格引用</dt><dd>{{ version.terms?.priceRef || '—' }}</dd></div>
      <div><dt>内容 SHA256</dt><dd class="mono break-all">{{ version.contentSha256 || '草稿未固定' }}</dd></div>
      <div><dt>创建时间</dt><dd>{{ formatTime(version.createdAt) }}</dd></div>
      <div><dt>发布/停售</dt><dd>{{ formatTime(version.retiredAt || version.publishedAt) }}</dd></div>
      <div><dt>最后原因</dt><dd>{{ version.reason || '—' }}</dd></div>
    </dl>

    <section class="terms">
      <h3>销售范围</h3>
      <div class="chips">
        <span v-for="scope in version.terms?.salesScope ?? []" :key="scope" class="chip">{{ scope }}</span>
        <span v-if="!version.terms?.salesScope?.length">—</span>
      </div>
    </section>

    <section class="terms">
      <h3>能力矩阵</h3>
      <article v-for="module in version.terms?.modules ?? []" :key="module.moduleCode" class="module-term">
        <div class="module-title"><strong>{{ moduleName(module.moduleCode) }}</strong><span class="mono">{{ module.moduleCode }}</span></div>
        <div class="term-columns">
          <div>
            <small>能力</small>
            <div class="chips"><span v-for="capability in module.capabilityCodes" :key="capability" class="chip">{{ capability }}</span><span v-if="!module.capabilityCodes?.length">—</span></div>
          </div>
          <div>
            <small>额度</small>
            <ul><li v-for="quota in module.quotas" :key="quota.key"><span class="mono">{{ quota.key }}</span> = {{ quota.unlimited ? 'unlimited' : quota.value }}</li><li v-if="!module.quotas?.length">—</li></ul>
          </div>
          <div>
            <small>字段策略</small>
            <ul><li v-for="field in module.fields" :key="`${field.key}:${field.action}`"><span class="mono">{{ field.key }}</span> · {{ field.action }} → {{ field.mode }}</li><li v-if="!module.fields?.length">—</li></ul>
          </div>
        </div>
      </article>
      <p v-if="!version.terms?.modules?.length" class="muted">当前版本没有模块条款。</p>
    </section>
  </section>
</template>

<style scoped>
.plan-detail { overflow: hidden; }
.detail-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--color-border); flex-wrap: wrap; }
.detail-header h2 { margin: 0; font-size: 16px; }
.detail-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.title-row, .actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.actions { justify-content: flex-end; }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: white; }
.facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin: 0; padding: 16px 20px; gap: 14px 20px; border-bottom: 1px solid var(--color-border); }
.facts div { min-width: 0; }
.facts dt { font-size: 11px; color: var(--color-text-muted); }
.facts dd { margin: 4px 0 0; font-size: 13px; }
.terms { padding: 16px 20px; border-bottom: 1px solid var(--color-border); }
.terms:last-child { border-bottom: 0; }
.terms h3 { margin: 0 0 10px; font-size: 13px; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chip { display: inline-flex; padding: 3px 7px; border-radius: 999px; background: var(--color-primary-soft); font-size: 11px; }
.module-term { padding: 12px 0; border-top: 1px solid var(--color-border); }
.module-term:first-of-type { border-top: 0; padding-top: 0; }
.module-title { display: flex; justify-content: space-between; gap: 10px; margin-bottom: 10px; }
.term-columns { display: grid; grid-template-columns: 1fr 1fr 1.2fr; gap: 12px; font-size: 12px; }
.term-columns small { color: var(--color-text-muted); }
.term-columns ul { margin: 6px 0 0; padding-left: 16px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.break-all { overflow-wrap: anywhere; }
.muted { color: var(--color-text-muted); font-size: 12px; }
@media (max-width: 760px) {
  .facts, .term-columns { grid-template-columns: 1fr; }
}
</style>
