<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { UiButton } from '@/ui/base'
import PlanChangeLifecycle from '@/features/enterprise/components/PlanChangeLifecycle.vue'
import type { EntitlementDecisionDTO } from '@/services/commercial/platformCommercial'
import {
  enterprisePlanRuntimeError,
  loadEnterprisePlanReadModel,
  type EnterprisePlanReadModel,
  type EnterpriseQuotaUsage,
} from '@/services/enterprise/planRuntime'

const loading = ref(true)
const errorMessage = ref('')
const model = ref<EnterprisePlanReadModel | null>(null)
const tab = ref<'overview' | 'modules' | 'quotas'>('overview')

const subscription = computed(() => model.value?.subscription ?? null)
const entitlements = computed(() => model.value?.entitlements ?? null)
const decisions = computed(() => entitlements.value?.decisions ?? [])
const moduleDecisions = computed(() => decisions.value.filter((item) => item.kind === 'module'))
const capabilityDecisions = computed(() => decisions.value.filter((item) => item.kind === 'capability'))
const quotaDecisions = computed(() => decisions.value.filter((item) => item.kind === 'quota'))
const enabledModules = computed(() => moduleDecisions.value.filter((item) => item.allowed).length)
const enabledCapabilities = computed(() => capabilityDecisions.value.filter((item) => item.allowed).length)
const usageItems = computed(() => model.value?.usage.usages ?? [])
const knownUsageCount = computed(() => usageItems.value.filter((item) => item.known).length)
const usageByKey = computed(() => {
  const map = new Map<string, EnterpriseQuotaUsage>()
  for (const item of usageItems.value) {
    if (!item.known) continue
    map.set(`${item.moduleCode}:${item.key}`, item)
  }
  return map
})

function decisionLabel(decision: EntitlementDecisionDTO) {
  return decision.key || decision.moduleCode || '未命名权益'
}

function moduleLabel(decision: EntitlementDecisionDTO) {
  return decision.moduleCode || decision.key || '未命名模块'
}

function usageFor(decision: EntitlementDecisionDTO) {
  return usageByKey.value.get(`${decision.moduleCode}:${decision.key}`)
}

function finiteLimit(decision: EntitlementDecisionDTO) {
  if (!decision.limit || decision.limit.unlimited) return null
  const value = Number(decision.limit.value)
  return Number.isFinite(value) && value >= 0 ? value : null
}

function usedNumber(decision: EntitlementDecisionDTO) {
  const usage = usageFor(decision)
  if (!usage?.known) return null
  const value = Number(usage.used)
  return Number.isFinite(value) && value >= 0 ? value : null
}

function limitLabel(decision: EntitlementDecisionDTO) {
  if (!decision.allowed) return '未开放'
  if (!decision.limit) return '未声明'
  if (decision.limit.unlimited) return '无限'
  return String(decision.limit.value ?? '—')
}

function usedLabel(decision: EntitlementDecisionDTO) {
  const used = usedNumber(decision)
  return used == null ? '未知' : String(used)
}

function remainingLabel(decision: EntitlementDecisionDTO) {
  if (!decision.allowed) return '—'
  if (decision.limit?.unlimited) return '无限'
  const limit = finiteLimit(decision)
  const used = usedNumber(decision)
  if (limit == null || used == null) return '未知'
  return String(Math.max(0, limit - used))
}

function quotaStatus(decision: EntitlementDecisionDTO) {
  if (!decision.allowed) return '未开放'
  if (decision.limit?.unlimited) return '无限额度'
  const limit = finiteLimit(decision)
  const used = usedNumber(decision)
  if (limit == null) return '额度未声明'
  if (used == null) return '用量未知'
  return used >= limit ? '额度已用尽' : '额度可用'
}

function quotaTone(decision: EntitlementDecisionDTO) {
  const state = quotaStatus(decision)
  if (state === '额度可用' || state === '无限额度') return 'success'
  if (state === '额度已用尽' || state === '未开放') return 'warning'
  return 'neutral'
}

function formatTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    model.value = await loadEnterprisePlanReadModel()
  } catch (error) {
    model.value = null
    errorMessage.value = enterprisePlanRuntimeError(error)
  } finally {
    loading.value = false
  }
}

async function refreshAfterChange() {
  try {
    model.value = await loadEnterprisePlanReadModel()
  } catch (error) {
    errorMessage.value = enterprisePlanRuntimeError(error)
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="page-stack" data-enterprise-plan-source="server">
    <PageHeading
      title="套餐额度"
      description="当前订阅、功能权益、额度用量与套餐变更均由服务端权威链路驱动"
    />

    <div v-if="loading" class="card state-card" role="status">正在读取当前租户套餐、权益与用量…</div>
    <div v-else-if="errorMessage && !model" class="card state-card error-state" role="alert">
      <div>
        <strong>无法读取套餐额度</strong>
        <p>{{ errorMessage }}</p>
      </div>
      <UiButton class="btn" @click="load">重新读取</UiButton>
    </div>

    <template v-else-if="subscription && entitlements && model">
      <div class="source-bar">
        <span><AppIcon name="shield" :size="15" /> Tenant Commercial API</span>
        <span>租户 {{ model.session.active_tenant_id }}</span>
        <span>权益版本 {{ entitlements.entitlementVersion }}</span>
        <span>已接入用量 {{ knownUsageCount }} 项</span>
        <span v-if="subscription.pendingChangeId">待处理 {{ subscription.pendingChangeId }}</span>
      </div>

      <div v-if="errorMessage" class="card inline-error" role="alert">
        <span>{{ errorMessage }}</span><UiButton class="btn" @click="refreshAfterChange">重新读取权威状态</UiButton>
      </div>

      <div class="plan-top">
        <section class="card current-plan">
          <div class="row-between plan-heading">
            <div class="row">
              <span class="plan-crown"><AppIcon name="crown" :size="28" /></span>
              <div>
                <small class="muted">当前订阅</small>
                <h2>{{ subscription.planCode }}</h2>
              </div>
            </div>
            <StatusBadge :text="subscription.state || 'unknown'" />
          </div>

          <div class="plan-meta-grid">
            <div><span>套餐版本</span><strong>v{{ subscription.planVersion }}</strong></div>
            <div><span>生效日期</span><strong>{{ formatTime(subscription.periodStart || subscription.createdAt) }}</strong></div>
            <div><span>到期日期</span><strong>{{ formatTime(subscription.periodEnd) }}</strong></div>
            <div><span>订阅修订</span><strong>r{{ subscription.revision }}</strong></div>
          </div>

          <div class="read-boundary">
            <AppIcon name="help" :size="16" />
            套餐切换、续订、停止续费已接入 tenant preview / confirm / receipt 权威链路；存在价格引用时必须等待外部商业或支付审批，页面不会自行认定已付款。
          </div>
        </section>

        <section class="card authority-summary">
          <div class="row-between">
            <h2>权威权益摘要</h2>
            <span class="muted">source v{{ entitlements.sourceVersion }}</span>
          </div>
          <div class="summary-grid">
            <div><span>已开放模块</span><strong>{{ enabledModules }}</strong><small>/ {{ moduleDecisions.length }}</small></div>
            <div><span>已开放能力</span><strong>{{ enabledCapabilities }}</strong><small>/ {{ capabilityDecisions.length }}</small></div>
            <div><span>额度项</span><strong>{{ quotaDecisions.length }}</strong><small>项</small></div>
            <div><span>权威用量</span><strong>{{ knownUsageCount }}</strong><small>项已接入</small></div>
          </div>
          <p v-if="model.usageError" class="usage-note usage-warning">
            用量服务暂不可用：{{ model.usageError }}。未取得权威 meter 的额度保持“未知”，不会显示为 0。
          </p>
          <p v-else class="usage-note">
            仅已绑定权威 meter 的 quota 展示实际使用量；当前成员额度由 Access 统计 invited / active / suspended，removed 才释放额度。其余未接入 meter 的额度仍显示“未知”。
          </p>
        </section>
      </div>

      <PlanChangeLifecycle
        :session="model.session"
        :subscription="subscription"
        @changed="refreshAfterChange"
      />

      <section class="card panel-pad">
        <div class="tabs">
          <UiButton :class="['tab', { active: tab === 'overview' }]" @click="tab = 'overview'">套餐概览</UiButton>
          <UiButton :class="['tab', { active: tab === 'modules' }]" @click="tab = 'modules'">功能权益</UiButton>
          <UiButton :class="['tab', { active: tab === 'quotas' }]" @click="tab = 'quotas'">额度用量</UiButton>
        </div>

        <div v-if="tab === 'overview'" class="overview-grid">
          <div class="overview-block"><span>订阅 ID</span><strong>{{ subscription.subscriptionId }}</strong></div>
          <div class="overview-block"><span>销售范围</span><strong>{{ subscription.salesScope || '—' }}</strong></div>
          <div class="overview-block"><span>续费状态</span><strong>{{ subscription.renewalStopped ? '已停止续费' : '按当前订阅策略' }}</strong></div>
          <div class="overview-block"><span>待生效变更</span><strong>{{ subscription.pendingChangeId || '无' }}</strong></div>
          <div class="overview-block wide"><span>最近权益求值</span><strong>{{ entitlements.evaluatedAt || '—' }}</strong></div>
          <div class="overview-block wide"><span>权限指纹</span><strong>{{ entitlements.permissionVersion || '—' }}</strong></div>
        </div>

        <div v-else-if="tab === 'modules'" class="feature-grid">
          <div v-for="decision in moduleDecisions" :key="`${decision.moduleCode}:${decision.key}`" class="feature-row">
            <span class="feature-icon"><AppIcon name="shield" :size="21" /></span>
            <div class="flex-1">
              <h3>{{ moduleLabel(decision) }}</h3>
              <p>{{ decision.reason || decisionLabel(decision) }}</p>
            </div>
            <StatusBadge :text="decision.allowed ? '已开放' : '未开放'" :tone="decision.allowed ? 'success' : 'warning'" :dot="false" />
          </div>
          <div v-if="moduleDecisions.length === 0" class="empty-row">服务端当前未返回模块级权益决策。</div>
        </div>

        <div v-else class="table-scroll">
          <table class="data-table quota-table">
            <thead>
              <tr>
                <th>模块</th>
                <th>额度键</th>
                <th>服务端上限</th>
                <th>已使用</th>
                <th>剩余</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="decision in quotaDecisions" :key="`${decision.moduleCode}:${decision.key}`">
                <td>{{ decision.moduleCode || '—' }}</td>
                <td>{{ decisionLabel(decision) }}</td>
                <td>{{ limitLabel(decision) }}</td>
                <td>
                  <strong v-if="usageFor(decision)?.known" class="usage-known">{{ usedLabel(decision) }}</strong>
                  <span v-else class="usage-unknown">未知</span>
                </td>
                <td>{{ remainingLabel(decision) }}</td>
                <td><StatusBadge :text="quotaStatus(decision)" :tone="quotaTone(decision)" :dot="false" /></td>
              </tr>
              <tr v-if="quotaDecisions.length === 0"><td colspan="6" class="empty-row">服务端当前未返回 quota entitlement。</td></tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.state-card { min-height: 150px; padding: 28px; display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.error-state strong { color: var(--color-danger); }
.error-state p { margin-top: 8px; color: var(--color-text-secondary); }
.inline-error { padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--color-danger); }
.source-bar { display: flex; flex-wrap: wrap; gap: 12px 20px; align-items: center; padding: 10px 14px; border: 1px solid var(--color-border); border-radius: 10px; color: var(--color-text-secondary); font-size: 12px; }
.source-bar span { display: inline-flex; align-items: center; gap: 6px; }
.plan-top { display: grid; grid-template-columns: 1fr 1.05fr; gap: 16px; }
.current-plan, .authority-summary { padding: 28px; }
.plan-crown { width: 58px; height: 58px; display: grid; place-items: center; border-radius: 50%; background: linear-gradient(130deg, var(--color-gradient-end), var(--color-primary)); color: var(--color-on-primary); }
.plan-heading h2 { margin-top: 4px; font-size: 24px; overflow-wrap: anywhere; }
.plan-meta-grid, .summary-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin-top: 26px; }
.plan-meta-grid span, .summary-grid span, .overview-block span { display: block; color: var(--color-text-muted); font-size: 12px; margin-bottom: 7px; }
.plan-meta-grid strong, .overview-block strong { font-size: 14px; font-weight: 550; overflow-wrap: anywhere; }
.summary-grid strong { font-size: 26px; }
.summary-grid small { margin-left: 4px; color: var(--color-text-muted); }
.read-boundary, .usage-note { margin-top: 24px; padding: 12px 14px; border-radius: 10px; background: var(--color-primary-soft); color: var(--color-text-secondary); font-size: 12px; line-height: 1.65; }
.read-boundary { display: flex; gap: 8px; align-items: flex-start; }
.usage-note { background: var(--color-surface-muted); }
.usage-warning { border: 1px solid var(--color-warning-border, var(--color-border)); }
.panel-pad { padding: 24px; }
.tabs { display: flex; gap: 8px; border-bottom: 1px solid var(--color-border); padding-bottom: 12px; margin-bottom: 22px; }
.tab { border-color: transparent; }
.tab.active { color: var(--color-primary); background: var(--color-primary-soft); }
.overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.overview-block { padding: 16px; background: var(--color-surface-muted); border-radius: 10px; min-width: 0; }
.feature-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.feature-row { display: flex; align-items: center; gap: 14px; padding: 16px; border: 1px solid var(--color-border); border-radius: 12px; min-width: 0; }
.feature-row h3 { font-size: 14px; overflow-wrap: anywhere; }
.feature-row p { margin-top: 4px; color: var(--color-text-muted); font-size: 12px; overflow-wrap: anywhere; }
.feature-icon { width: 38px; height: 38px; flex: 0 0 38px; border-radius: 10px; display: grid; place-items: center; color: var(--color-primary); background: var(--color-primary-soft); }
.usage-known { font-variant-numeric: tabular-nums; }
.usage-unknown { color: var(--color-text-muted); font-size: 12px; }
.empty-row { color: var(--color-text-muted); text-align: center; padding: 24px; }
.quota-table { min-width: 820px; }
@media (max-width: 900px) { .plan-top { grid-template-columns: 1fr; } .feature-grid { grid-template-columns: 1fr; } }
@media (max-width: 560px) { .current-plan, .authority-summary, .panel-pad { padding: 18px; } .plan-heading { align-items: flex-start; } .plan-meta-grid, .summary-grid, .overview-grid { grid-template-columns: 1fr; } .source-bar { align-items: flex-start; flex-direction: column; } .tabs { overflow-x: auto; } .tab { flex: 0 0 auto; } .inline-error { align-items: stretch; flex-direction: column; } }
</style>
