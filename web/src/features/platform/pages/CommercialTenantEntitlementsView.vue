<script setup lang="ts">
import { UiButton, UiInput, UiTextarea } from '@/ui/base'

import AuthorityPicker from '@/features/platform/components/AuthorityPicker.vue'
import { computed, onMounted, ref } from 'vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import EntitlementDecisionTable from '@/features/platform/components/EntitlementDecisionTable.vue'
import EntitlementOverrideDialog from '@/features/platform/components/EntitlementOverrideDialog.vue'
import EntitlementOverrideTable from '@/features/platform/components/EntitlementOverrideTable.vue'
import SubscriptionChangeWorkspace from '@/features/platform/components/SubscriptionChangeWorkspace.vue'
import {
  CommercialApiError,
  commercialRequestId,
  createEntitlementOverride,
  explainTenantEntitlements,
  getTenantSubscription,
  listEntitlementOverrides,
  listPlatformModules,
  revokeEntitlementOverride,
  type CreateEntitlementOverrideInput,
  type EntitlementOverrideDTO,
  type EntitlementView,
  type ModuleDTO,
  type TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'

type LoadState = 'idle' | 'loading' | 'ready' | 'blocked' | 'error'

const tenantIdInput = ref('')
const activeTenantId = ref('')
const capabilityFilter = ref('')
const loadState = ref<LoadState>('idle')
const errorMessage = ref('')
const actionMessage = ref('')
const actionError = ref('')
const pending = ref(false)

const modules = ref<ModuleDTO[]>([])
const moduleLoadError = ref('')
const subscription = ref<TenantSubscriptionDTO | null>(null)
const entitlement = ref<EntitlementView | null>(null)
const overrides = ref<EntitlementOverrideDTO[]>([])
const sourceVersion = ref<string | number>(0)

const overrideDialogOpen = ref(false)
const revokeTarget = ref<EntitlementOverrideDTO | null>(null)
const revokeReason = ref('')

const summary = computed(() => ({
  decisions: entitlement.value?.decisions?.length ?? 0,
  sourceVersion: entitlement.value?.sourceVersion ?? sourceVersion.value,
  entitlementVersion: entitlement.value?.entitlementVersion ?? '—',
  nextTransition: entitlement.value?.nextTransitionAt || '无计划切换',
}))

function capabilityCodes() {
  return capabilityFilter.value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function formatTime(value: string) {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? value : parsed.toLocaleString('zh-CN', { hour12: false })
}

function subscriptionStateLabel(value: string) {
  if (value === 'ACTIVE') return '有效'
  if (value === 'TRIAL') return '试用'
  if (value === 'GRACE_PERIOD') return '宽限期'
  if (value === 'SUSPENDED') return '已暂停'
  if (value === 'EXPIRED') return '已过期'
  return '待确认'
}

async function loadModules() {
  moduleLoadError.value = ''
  try {
    modules.value = await listPlatformModules()
  } catch (error) {
    modules.value = []
    moduleLoadError.value = error instanceof Error ? error.message : '模块目录读取失败'
  }
}

async function readSubscription(tenantId: string) {
  try {
    return await getTenantSubscription(tenantId)
  } catch (error) {
    if (error instanceof CommercialApiError && error.status === 404) return null
    throw error
  }
}

async function loadWorkspace() {
  const tenantId = tenantIdInput.value.trim()
  if (!tenantId) {
    loadState.value = 'idle'
    activeTenantId.value = ''
    return
  }

  loadState.value = 'loading'
  activeTenantId.value = tenantId
  errorMessage.value = ''
  actionError.value = ''
  actionMessage.value = ''

  try {
    const [subscriptionResult, overrideResult, entitlementResult] = await Promise.all([
      readSubscription(tenantId),
      listEntitlementOverrides(tenantId),
      explainTenantEntitlements(tenantId, capabilityCodes()),
    ])
    subscription.value = subscriptionResult
    overrides.value = overrideResult.sources
    sourceVersion.value = overrideResult.sourceVersion
    entitlement.value = entitlementResult
    loadState.value = 'ready'
  } catch (error) {
    if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
      loadState.value = 'blocked'
      errorMessage.value = error.message
      return
    }
    loadState.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '租户权益读取失败'
  }
}

async function refreshExplanation() {
  if (!activeTenantId.value) return
  pending.value = true
  actionError.value = ''
  try {
    entitlement.value = await explainTenantEntitlements(activeTenantId.value, capabilityCodes())
    actionMessage.value = '权益解释已按当前过滤条件重新计算。'
  } catch (error) {
    handleActionError(error, '权益解释刷新失败')
  } finally {
    pending.value = false
  }
}

async function createOverride(input: CreateEntitlementOverrideInput) {
  if (!activeTenantId.value) return
  pending.value = true
  actionError.value = ''
  try {
    await createEntitlementOverride(activeTenantId.value, {
      ...input,
      requestId: commercialRequestId('ce13-entitlement-override'),
      expectedVersion: sourceVersion.value,
    })
    overrideDialogOpen.value = false
    actionMessage.value = '专项权益已创建，正在刷新最新权益结果。'
    await loadWorkspace()
  } catch (error) {
    await handleActionError(error, '专项权益来源创建失败', true)
  } finally {
    pending.value = false
  }
}

function openRevoke(source: EntitlementOverrideDTO) {
  revokeTarget.value = source
  revokeReason.value = ''
  actionError.value = ''
}

async function confirmRevoke() {
  if (!activeTenantId.value || !revokeTarget.value) return
  if (!revokeReason.value.trim()) {
    actionError.value = '撤销原因不能为空。'
    return
  }

  pending.value = true
  actionError.value = ''
  try {
    await revokeEntitlementOverride(activeTenantId.value, revokeTarget.value.id, {
      requestId: commercialRequestId('ce13-entitlement-revoke'),
      expectedVersion: sourceVersion.value,
      reason: revokeReason.value.trim(),
    })
    revokeTarget.value = null
    actionMessage.value = '专项权益已撤销；历史记录已保留，最新权益结果已更新。'
    await loadWorkspace()
  } catch (error) {
    await handleActionError(error, '专项权益撤销失败', true)
  } finally {
    pending.value = false
  }
}

async function handleActionError(error: unknown, fallback: string, rereadOnConflict = false) {
  if (error instanceof CommercialApiError && error.code === 'conflict') {
    actionError.value = '权益信息已变化，本次操作没有覆盖其他人的修改；已重新加载最新结果。'
    if (rereadOnConflict) await loadWorkspace()
    return
  }
  if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
    actionError.value = `当前平台账号无权执行该操作：${error.message}`
    return
  }
  actionError.value = error instanceof Error ? error.message : fallback
}

onMounted(loadModules)
</script>

<template>
  <div class="page-stack" data-testid="ce13-tenant-entitlements" data-ui-template="WorkbenchPage">
    <div data-ui-region="page-heading">
      <PageHeading
        title="租户权益"
        description="查看当前租户的订阅、权益结果、专项权益与变更记录"
      />
    </div>

    <div data-ui-region="authority"><AuthorityPicker kind="tenants" @select="(id) => { tenantIdInput = id; loadWorkspace() }" /></div>
    <section class="card workspace-card" data-ui-region="context">
      <div class="workspace-title">
        <div><h2>租户权益工作台</h2><p>选择租户后查看当前套餐、专项权益与最终可用能力。</p></div>
        <span class="boundary-badge">平台管理</span>
      </div>
      <form class="lookup-row" @submit.prevent="loadWorkspace">
        <label for="tenant-id">租户编号</label>
        <UiInput id="tenant-id" v-model="tenantIdInput" class="input" autocomplete="off" placeholder="输入租户编号" />
        <UiButton class="btn primary" type="submit" :disabled="loadState === 'loading'">读取权益</UiButton>
      </form>
      <div class="filter-row">
        <label for="capability-filter">功能能力过滤（可选）</label>
        <UiInput id="capability-filter" v-model="capabilityFilter" class="input" placeholder="输入需要查看的功能能力" />
        <UiButton class="btn" type="button" :disabled="!activeTenantId || pending" @click="refreshExplanation">重新计算</UiButton>
      </div>
    </section>

    <section v-if="actionMessage" class="notice success" role="status">{{ actionMessage }}</section>
    <section v-if="actionError && !overrideDialogOpen" class="notice danger" role="alert">{{ actionError }}</section>

    <section v-if="loadState === 'idle'" class="card state-card"><strong>选择或输入租户编号开始</strong><p>可查看内容取决于当前平台账号的权限范围。</p></section>
    <section v-else-if="loadState === 'loading'" class="card state-card" aria-live="polite"><strong>正在读取 {{ activeTenantId }} 的权益信息</strong><p>正在获取当前订阅、专项权益和权益结果。</p></section>
    <section v-else-if="loadState === 'blocked'" class="card state-card warning" role="alert"><strong>当前平台账号无权读取该租户权益</strong><p>{{ errorMessage }}</p><UiButton class="btn" type="button" @click="loadWorkspace">重新检查</UiButton></section>
    <section v-else-if="loadState === 'error'" class="card state-card danger" role="alert"><strong>租户权益读取失败</strong><p>{{ errorMessage }}</p><UiButton class="btn" type="button" @click="loadWorkspace">重试</UiButton></section>

    <template v-else-if="loadState === 'ready' && entitlement">
      <section class="metric-grid" data-ui-region="metrics">
        <article class="card metric"><span>专项权益版本</span><strong>{{ summary.sourceVersion }}</strong><small>用于识别当前专项权益状态</small></article>
        <article class="card metric"><span>权益版本</span><strong>{{ summary.entitlementVersion }}</strong><small>当前计算版本</small></article>
        <article class="card metric"><span>权益决策</span><strong>{{ summary.decisions }}</strong><small>{{ capabilityCodes().length ? '已过滤' : '全部' }}</small></article>
        <article class="card metric"><span>下一变化时间</span><strong class="time-value">{{ formatTime(String(summary.nextTransition)) }}</strong><small>到达该时间后权益可能发生变化</small></article>
      </section>

      <section class="card subscription-card" data-ui-region="subscription">
        <div class="section-header"><div><h2>当前订阅</h2><p>订阅是权益来源之一，最终可用权益以当前结果为准。</p></div><StatusBadge v-if="subscription" :text="subscriptionStateLabel(subscription.state)" :tone="subscription.state === 'ACTIVE' ? 'success' : 'neutral'" /></div>
        <div v-if="subscription" class="subscription-grid">
          <div><span>套餐</span><strong>{{ subscription.planCode }} v{{ subscription.planVersion }}</strong></div>
          <div><span>销售范围</span><strong>{{ subscription.salesScope || '—' }}</strong></div>
          <div><span>期间</span><strong>{{ formatTime(subscription.periodStart) }} → {{ formatTime(subscription.periodEnd) }}</strong></div>
          <div><span>权益来源版本</span><strong>{{ subscription.entitlementSourceVersion }}</strong></div>
          <div><span>待处理变更</span><strong>{{ subscription.pendingChangeId ? '有待处理变更' : '无' }}</strong></div>
          <div><span>匹配说明</span><strong>{{ subscription.matchExplanation || '—' }}</strong></div>
        </div>
        <p v-else class="empty-text">当前没有可读取的租户订阅记录。</p>
      </section>

      <div data-ui-region="change-workspace"><SubscriptionChangeWorkspace :tenant-id="activeTenantId" :subscription="subscription" @refresh="loadWorkspace" /></div>

      <section class="resolver-meta card" data-ui-region="resolver-meta">
        <span>计算时间 {{ formatTime(entitlement.evaluatedAt) }}</span>
        <span>结果有效至 {{ formatTime(entitlement.validUntil) }}</span>
      </section>

      <div data-ui-region="decisions"><EntitlementDecisionTable :decisions="entitlement.decisions ?? []" /></div>
      <div data-ui-region="overrides"><EntitlementOverrideTable :sources="overrides" :source-version="sourceVersion" :pending="pending" @create="overrideDialogOpen = true" @revoke="openRevoke" /></div>
    </template>

    <EntitlementOverrideDialog
      :open="overrideDialogOpen"
      :source-version="sourceVersion"
      :modules="modules"
      :pending="pending"
      :error="actionError || moduleLoadError"
      @close="overrideDialogOpen = false"
      @submit="createOverride"
    />

    <div v-if="revokeTarget" class="dialog-backdrop" @click.self="revokeTarget = null">
      <section class="card revoke-dialog" role="dialog" aria-modal="true">
        <h2>撤销专项来源</h2>
        <p>撤销后该专项权益不再生效，历史操作记录仍会保留。</p>
        <label>撤销原因<UiTextarea v-model="revokeReason" class="input" rows="3" /></label>
        <div class="dialog-actions"><UiButton class="btn" type="button" @click="revokeTarget = null">取消</UiButton><UiButton class="btn primary" type="button" :disabled="pending" @click="confirmRevoke">确认撤销</UiButton></div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.workspace-card { padding: 20px; display: grid; gap: 15px; }
.workspace-title, .section-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-title h2, .section-header h2 { margin: 0; font-size: 16px; }
.workspace-title p, .section-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.boundary-badge { padding: 5px 8px; border-radius: 8px; background: var(--color-primary-soft); color: var(--color-primary); font-size: 11px; white-space: nowrap; }
.lookup-row, .filter-row { display: grid; grid-template-columns: 140px minmax(0, 1fr) auto; gap: 10px; align-items: center; }
.lookup-row label, .filter-row label { font-size: 12px; color: var(--color-text-secondary); }
.state-card { min-height: 112px; padding: 22px; display: flex; align-items: flex-start; gap: 14px; flex-wrap: wrap; }
.state-card p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; flex: 1 1 100%; }
.metric-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { padding: 18px; min-width: 0; }
.metric span, .metric small { display: block; color: var(--color-text-muted); font-size: 12px; }
.metric strong { display: block; margin: 7px 0 3px; font-size: 24px; overflow-wrap: anywhere; }
.metric .time-value { font-size: 13px; line-height: 1.5; }
.subscription-card { padding: 20px; }
.subscription-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 16px; }
.subscription-grid div { padding: 12px; border-radius: 9px; background: var(--color-surface-subtle); }
.subscription-grid span { display: block; color: var(--color-text-muted); font-size: 11px; }
.subscription-grid strong { display: block; margin-top: 5px; font-size: 13px; overflow-wrap: anywhere; }
.empty-text { color: var(--color-text-muted); font-size: 13px; }
.resolver-meta { display: flex; flex-wrap: wrap; gap: 7px 16px; padding: 12px 16px; color: var(--color-text-muted); font-size: 11px; }
.dialog-backdrop { position: fixed; inset: 0; z-index: 90; display: grid; place-items: center; padding: 20px; background: var(--color-fixed-99fdea0b); }
.revoke-dialog { width: min(480px, 100%); padding: 20px; }
.revoke-dialog h2 { margin: 0; font-size: 17px; }
.revoke-dialog p { color: var(--color-text-muted); font-size: 12px; }
.revoke-dialog label { display: grid; gap: 7px; font-size: 13px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
@media (max-width: 900px) { .metric-grid { grid-template-columns: repeat(2, 1fr); } .subscription-grid { grid-template-columns: 1fr 1fr; } }
@media (max-width: 680px) { .lookup-row, .filter-row { grid-template-columns: 1fr; } .metric-grid, .subscription-grid { grid-template-columns: 1fr; } }
</style>
