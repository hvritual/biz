<script setup lang="ts">
import { UiButton, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useEnterprisePlanStore } from '@/stores/enterprisePlan'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/ui/common/PageHeading.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import PlanChangeLifecycle from '@/features/enterprise/components/PlanChangeLifecycle.vue'
import {
  currentAuthorizationMatchesSession, currentAuthorizationState,
  ensureCurrentAuthorization, redirectToTrustedLogin,
} from '@/services/runtime/authorization'
import { planChangeAccess } from '@/services/enterprise/planAccess'
import { listMySubscriptionChanges } from '@/services/enterprise/planChangeRuntime'
import type { SubscriptionChangeReceiptDTO } from '@/services/commercial/platformCommercial'
import { backendTermLabel } from '@/i18n/backend-terms'
import { currentUiLocale, t } from '@/i18n'

const store = useEnterpriseStore()
const plan = useEnterprisePlanStore()
const ui = useUiStore()
const route = useRoute()
const router = useRouter()
const requestOpen = ref(false)
const quotaExpansionOpen = ref(false)
const quotaExpansionKey = ref('')
const lifecycleOpen = ref(false)
const targetPlan = ref('企业版')
const note = ref('')
const tab = ref('套餐概览')
const changeHistory = ref<SubscriptionChangeReceiptDTO[]>([])
const historyLoading = ref(false)
const historyError = ref(false)
const checkingPermissions = ref(false)
let historyGeneration = 0

const access = computed(() => store.previewMode ? 'allowed' : planChangeAccess(
  currentAuthorizationState.snapshot, store.session, currentAuthorizationState.status,
  currentAuthorizationMatchesSession(store.session),
))
const canChangePlan = computed(() => access.value === 'allowed' && plan.canUseCurrentFacts)
const tenantLabel = computed(() => store.tenantOptions.find((tenant) => tenant.id === store.tenantId)?.name || store.tenantId || '—')
const enabledCount = computed(() => plan.features.filter((feature) => feature.enabled).length)
const quotaCards = computed(() => plan.quotas.slice(0, 4))
const nearLimitQuotas = computed(() => plan.quotas.filter((quota) =>
  !quota.unlimited && quota.used != null && quota.total != null && quota.total > 0 && quota.used / quota.total >= 0.8,
))
const expansionQuota = computed(() => plan.quotas.find((quota) => quota.key === quotaExpansionKey.value))
const feedbackTitle = computed(() => {
  if (plan.readIssue === 'denied') return t('planFeedback.readDeniedTitle')
  if (plan.readIssue === 'login') return t('planFeedback.access.login')
  if (plan.readIssue === 'context') return t('planFeedback.contextChangedTitle')
  if (plan.readIssue === 'usage') return t('planFeedback.usageTitle')
  return t(plan.stale ? 'planFeedback.staleTitle' : 'planFeedback.readFailedTitle')
})
const feedbackBody = computed(() => {
  if (plan.readIssue === 'denied') return t('planFeedback.readDeniedBody')
  if (plan.readIssue === 'context') return t('planFeedback.contextChangedBody')
  if (plan.readIssue === 'login') return t('planFeedback.access.login')
  if (plan.readIssue === 'usage') return t('planFeedback.usageBody')
  return t(plan.stale ? 'planFeedback.staleBody' : 'planFeedback.readFailedBody')
})

watch(planScope, () => {
  historyGeneration += 1
  changeHistory.value = []
  historyError.value = false
  historyLoading.value = false
  lifecycleOpen.value = false
  requestOpen.value = false
  quotaExpansionOpen.value = false
  quotaExpansionKey.value = ''
  note.value = ''
  tab.value = '套餐概览'
}, { flush: 'sync' })
function planScope() { return plan.scopeKey }

watch([() => route.query.action, canChangePlan, () => access.value], ([action, allowed, state]) => {
  if (action !== 'upgrade' || state === 'checking' || plan.loading) return
  if (allowed) openChange()
  void router.replace({ path: route.path, query: { ...route.query, action: undefined } })
}, { immediate: true })
watch(tab, (value) => {
  if (value === '变更记录' && plan.isServerBacked) void loadChangeHistory()
})
watch(() => plan.serverChangeContext?.subscription.pendingChangeId, (id) => {
  if (plan.isServerBacked && id && access.value === 'allowed') lifecycleOpen.value = true
}, { immediate: true })

async function loadChangeHistory() {
  const context = plan.serverChangeContext
  if (!context || historyLoading.value || access.value !== 'allowed') return
  const key = plan.scopeKey
  const ticket = ++historyGeneration
  historyLoading.value = true
  historyError.value = false
  try {
    const result = await listMySubscriptionChanges(context.session)
    if (key !== plan.scopeKey || ticket !== historyGeneration) return
    if (result.receipts.some((receipt) => receipt.tenantId !== store.tenantId)) throw new Error('History scope mismatch')
    changeHistory.value = result.receipts
  } catch {
    if (key === plan.scopeKey && ticket === historyGeneration) historyError.value = true
  } finally {
    if (key === plan.scopeKey && ticket === historyGeneration) historyLoading.value = false
  }
}

async function recheckAccess() {
  if (checkingPermissions.value) return
  checkingPermissions.value = true
  try {
    await ensureCurrentAuthorization(true)
    await plan.load()
  } finally {
    checkingPermissions.value = false
  }
}
function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat(currentUiLocale(), { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date)
}
function formatReadTime(value: string) {
  return value ? new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—'
}
function quotaUsedLabel(used: number | null, unit: string) { return used == null ? t('planFeedback.unknown') : `${used}${unit ? ` ${unit}` : ''}` }
function quotaTotalLabel(total: number | null, unlimited: boolean, unit: string) { return unlimited ? '无限' : total == null ? '未声明' : `${total}${unit ? ` ${unit}` : ''}` }
function quotaPercent(used: number | null, total: number | null) { return used == null || total == null || total <= 0 ? 0 : Math.min(100, used / total * 100) }
function openQuotaExpansion(key: string) {
  if (!plan.canUseCurrentFacts) return
  quotaExpansionKey.value = key
  quotaExpansionOpen.value = true
}
function openChange() {
  if (!canChangePlan.value) return
  if (plan.isServerBacked) lifecycleOpen.value = true
  else requestOpen.value = true
}
function submitDemoChange() {
  try {
    plan.recordDemoChange(targetPlan.value, note.value)
    requestOpen.value = false
    ui.toast('已记录预览申请；未变更真实套餐，也不会发起扣费。', 'info')
  } catch {
    ui.toast('套餐调整申请失败。', 'error')
  }
}
</script>

<template>
  <div class="page-stack" data-enterprise-page="plan" data-ui-template="WorkbenchPage">
    <PageHeading title="套餐信息" description="查看当前订阅、可用功能与使用额度，让业务增长有据可依" />
    <div class="plan-context" data-plan-context>
      <span>{{ t('planFeedback.context') }}：<strong>{{ tenantLabel }}</strong></span>
      <span v-if="plan.lastReadAt">{{ t('planFeedback.lastRead') }}：{{ formatReadTime(plan.lastReadAt) }}</span>
      <UiButton v-if="plan.model" class="btn" :disabled="plan.loading" @click="plan.load">{{ t(plan.loading ? 'planFeedback.retrying' : 'planFeedback.retry') }}</UiButton>
    </div>
    <section v-if="plan.isServerBacked && plan.readIssue" class="card state-card error-state" role="alert" data-plan-read-feedback :data-issue="plan.readIssue" :aria-busy="plan.loading">
      <AppIcon :name="plan.readIssue === 'denied' ? 'lock' : 'warning'" :size="24" />
      <div class="flex-1">
        <h2>{{ feedbackTitle }}</h2>
        <p>{{ feedbackBody }}</p>
        <small>{{ t('planFeedback.retryHint') }}</small>
      </div>
      <UiButton v-if="plan.readIssue === 'login'" class="btn btn-primary" @click="redirectToTrustedLogin">{{ t('planFeedback.login') }}</UiButton>
      <UiButton v-else class="btn" :disabled="plan.loading || checkingPermissions" @click="plan.readIssue === 'denied' ? recheckAccess() : plan.load()">{{ t(plan.loading ? 'planFeedback.retrying' : 'planFeedback.retry') }}</UiButton>
    </section>
    <div v-else-if="plan.isServerBacked && !plan.model" class="card state-card" role="status">{{ t('planFeedback.checking') }}</div>

    <section v-if="plan.isServerBacked && access !== 'allowed'" class="card access-card" role="status" data-plan-access-state :data-reason="access">
      <AppIcon name="lock" :size="24" />
      <div class="flex-1"><h2>{{ t('planFeedback.permissionTitle') }}</h2><p>{{ t(`planFeedback.access.${access}`) }}</p></div>
      <div class="feedback-actions">
        <UiButton v-if="access === 'login'" class="btn" @click="redirectToTrustedLogin">{{ t('planFeedback.login') }}</UiButton>
        <UiButton v-else class="btn" :disabled="checkingPermissions || access === 'checking'" @click="recheckAccess">{{ t('planFeedback.recheck') }}</UiButton>
        <UiButton v-if="access === 'entitlement' && plan.model" class="btn" @click="tab = '功能权益'">{{ t('planFeedback.viewFeatures') }}</UiButton>
      </div>
    </section>

    <template v-if="!plan.isServerBacked || plan.model">
      <div class="plan-top">
        <section class="card current-plan" data-ui-region="subscription">
          <div class="row">
            <span class="plan-crown"><AppIcon name="crown" :size="30" /></span>
            <div><small class="muted">当前套餐</small><h2>{{ plan.currentPlan }} <StatusBadge :text="plan.subscriptionState" :tone="plan.subscriptionTone" /></h2></div>
          </div>
          <p>{{ plan.isServerBacked ? '查看当前套餐、功能权益与可用额度。' : '适用于多点位运营团队，统一管理设备、成员与服务。' }}</p>
          <div class="plan-dates">
            <div><span>生效日期</span><strong>{{ formatDate(plan.periodStart) }}</strong></div>
            <div><span>到期日期</span><strong>{{ formatDate(plan.periodEnd) }}</strong></div>
            <div><span>订阅周期</span><strong>{{ plan.cycle }}</strong></div>
          </div>
          <div class="row wrap">
            <UiButton class="btn btn-primary" :disabled="!canChangePlan" @click="openChange"><AppIcon name="crown" :size="16" />{{ plan.isServerBacked ? '管理套餐变更' : '申请升级套餐' }}</UiButton>
            <UiButton class="btn" :disabled="plan.isServerBacked && access !== 'allowed'" @click="tab = '变更记录'">{{ t('planFeedback.viewHistory') }}</UiButton>
          </div>
          <small class="preview-plan">{{ plan.isServerBacked ? '套餐变更将在确认后按业务规则处理，最终状态以处理结果为准。' : '当前套餐信息仅供查看。' }}</small>
        </section>
        <section class="card quota-overview" data-ui-region="quota-summary">
          <div class="row-between"><h2>额度使用概览</h2><span class="muted">当前企业</span></div>
          <div class="quota-cards">
            <div v-for="quota in quotaCards" :key="quota.key" class="quota-item">
              <div class="row-between"><span class="row"><AppIcon :name="quota.icon" :size="17" />{{ quota.label }}</span><span class="muted">{{ quota.status }}</span></div>
              <strong class="numeric">{{ quotaUsedLabel(quota.used, quota.unit) }} <small>/ {{ quotaTotalLabel(quota.total, quota.unlimited, quota.unit) }}</small></strong>
              <div v-if="quota.used != null && quota.total != null && quota.total > 0" class="progress-track"><div class="progress-fill" :style="{ width: quotaPercent(quota.used, quota.total) + '%' }" /></div>
            </div>
            <div v-if="quotaCards.length === 0" class="muted">当前套餐未返回可展示的额度项。</div>
          </div>
          <div v-if="nearLimitQuotas.length && plan.canUseCurrentFacts" class="quota-attention">
            <div><strong>{{ t('planFeedback.pressure') }}</strong><p>{{ t('planFeedback.pressureBody') }}</p></div>
            <UiButton class="btn" @click="openQuotaExpansion(nearLimitQuotas[0]!.key)">查看扩容规划</UiButton>
          </div>
        </section>
      </div>
      <PlanChangeLifecycle v-if="lifecycleOpen && plan.serverChangeContext" :session="plan.serverChangeContext.session" :subscription="plan.serverChangeContext.subscription" :blocked="!canChangePlan" @changed="plan.refreshAfterChange" />
      <section class="card panel-pad" data-ui-region="workspace">
        <div class="tabs" aria-label="套餐信息">
          <UiButton v-for="item in ['套餐概览', '功能权益', '使用额度', '变更记录']" :key="item" :role="item === '变更记录' ? 'tab' : undefined" :aria-selected="item === '变更记录' ? tab === item : undefined" :class="['tab', { active: tab === item }]" @click="tab = item">{{ item }}</UiButton>
        </div>
        <template v-if="tab === '套餐概览' || tab === '功能权益'">
          <div class="row-between plan-section-heading"><h2>套餐权益</h2><span class="muted">已开通 {{ enabledCount }} / {{ plan.features.length }} 项</span></div>
          <div class="feature-grid">
            <div v-for="feature in plan.features" :key="feature.key" class="feature-row">
              <span class="feature-icon"><AppIcon :name="feature.icon" :size="22" /></span>
              <div class="flex-1"><h3>{{ feature.label }}</h3><p>{{ feature.description }}</p></div>
              <StatusBadge :text="feature.enabled ? '已开通' : t('planFeedback.unavailable')" :tone="feature.enabled ? 'success' : 'warning'" :dot="false" />
            </div>
            <div v-if="plan.features.length === 0" class="muted">当前套餐暂无可展示的模块权益。</div>
          </div>
        </template>
        <template v-else-if="tab === '使用额度'">
          <div class="table-scroll quota-table">
            <table class="data-table">
              <thead><tr><th>资源类型</th><th>已使用</th><th>总额度</th><th>使用率</th><th>状态</th></tr></thead>
              <tbody>
                <tr v-for="quota in plan.quotas" :key="quota.key">
                  <td>{{ quota.label }}</td><td>{{ quotaUsedLabel(quota.used, quota.unit) }}</td><td>{{ quotaTotalLabel(quota.total, quota.unlimited, quota.unit) }}</td>
                  <td>{{ quota.used != null && quota.total != null && quota.total > 0 ? quotaPercent(quota.used, quota.total).toFixed(1) + '%' : '—' }}</td>
                  <td><StatusBadge :text="quota.status" :tone="quota.status === '正常' || quota.status === '无限额度' ? 'success' : 'warning'" /></td>
                </tr>
                <tr v-if="plan.quotas.length === 0"><td colspan="5" class="muted">当前套餐未返回额度项。</td></tr>
              </tbody>
            </table>
          </div>
        </template>
        <template v-else>
          <div v-if="historyError" class="notice-box warning" role="alert"><div><p>{{ t('planFeedback.historyError') }}</p><small v-if="changeHistory.length">{{ t('planFeedback.staleHistory') }}</small></div><UiButton class="btn" :disabled="historyLoading" @click="loadChangeHistory">{{ t('planFeedback.historyRetry') }}</UiButton></div>
          <div class="timeline plan-timeline">
            <template v-if="plan.isServerBacked && plan.model">
              <div v-if="historyLoading" class="timeline-item" role="status">正在读取套餐变更记录…</div>
              <p v-else-if="access !== 'allowed'">{{ t('planFeedback.permissionTitle') }}</p>
              <template v-else-if="changeHistory.length">
                <div v-for="receipt in changeHistory" :key="receipt.changeId" class="timeline-item"><strong>{{ backendTermLabel('changeAction', receipt.action) }} · {{ backendTermLabel('receiptStatus', receipt.status) }}</strong><p>{{ backendTermLabel('plan', receipt.before?.planCode) }} → {{ backendTermLabel('plan', receipt.after?.planCode) }}</p><small>{{ receipt.confirmedAt }} · {{ receipt.changeId }}</small></div>
              </template>
              <div v-else-if="!historyError" class="timeline-item"><strong>暂无套餐变更记录</strong><small>新的套餐变更将在此显示其申请、确认和生效状态。</small></div>
            </template>
            <template v-else><div v-for="log in store.logs.filter((item) => item.module === '套餐信息')" :key="log.id" class="timeline-item"><strong>{{ log.action }}</strong><p>{{ log.target }} · {{ log.after }}</p><small>{{ log.time }}</small></div><div class="timeline-item"><strong>标准版预览套餐初始化</strong><small>2026-09-08 · 示例数据</small></div></template>
          </div>
        </template>
      </section>
    </template>
    <UiDialog :open="quotaExpansionOpen && plan.canUseCurrentFacts" title="额度扩容规划" width="720px" @close="quotaExpansionOpen = false">
      <div class="page-stack">
        <StatusBadge text="目标设计 · 待接入" tone="warning" />
        <p>{{ t('planFeedback.expansionBody') }}</p>
        <div class="expansion-summary"><div><span>当前企业</span><strong>{{ tenantLabel }}</strong></div><div><span>扩容对象</span><strong>{{ expansionQuota?.label || '—' }}</strong></div><div><span>当前使用</span><strong>{{ expansionQuota ? quotaUsedLabel(expansionQuota.used, expansionQuota.unit) : '—' }}</strong></div><div><span>当前上限</span><strong>{{ expansionQuota ? quotaTotalLabel(expansionQuota.total, expansionQuota.unlimited, expansionQuota.unit) : '—' }}</strong></div></div>
        <label class="field"><span>扩容类型</span><UiSelect v-model="quotaExpansionKey" class="select"><UiOption v-for="quota in plan.quotas" :key="quota.key" :value="quota.key">{{ quota.label }}</UiOption></UiSelect><small>客户、成员及设备容量分别计算，不作为通用积分。</small></label>
        <p class="notice-box warning">{{ t('planFeedback.expansionPending') }}</p>
      </div>
      <template #footer><UiButton class="btn" @click="quotaExpansionOpen = false">返回使用额度</UiButton><UiButton class="btn btn-primary" disabled>获取报价 · 待接入</UiButton></template>
    </UiDialog>
    <UiDialog :open="requestOpen" title="申请套餐调整" @close="requestOpen = false">
      <div class="page-stack"><div class="notice-box"><AppIcon name="help" />此操作仅用于演示申请流程，不会购买服务、变更实际订阅或扣费。</div><label class="field"><span>意向套餐</span><UiSelect v-model="targetPlan" class="select"><UiOption>企业版</UiOption><UiOption>标准版扩容</UiOption><UiOption>联系商务定制</UiOption></UiSelect></label><label class="field"><span>需求说明</span><UiTextarea v-model="note" class="textarea" maxlength="500" placeholder="描述所需成员、点位、设备或功能额度" /></label></div>
      <template #footer><UiButton class="btn" @click="requestOpen = false">取消</UiButton><UiButton class="btn btn-primary" @click="submitDemoChange">记录申请</UiButton></template>
    </UiDialog>
  </div>
</template>

<style scoped>
.plan-context { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 24px; color: var(--color-text-secondary); font-size: var(--text-sm); overflow-wrap: anywhere; }
.plan-context .btn { margin-inline-start: auto; }
.state-card, .access-card { padding: 24px; display: flex; align-items: flex-start; gap: 16px; }
.state-card h2, .access-card h2 { font-size: var(--text-lg); line-height: 1.5; }
.state-card p, .access-card p { margin: 6px 0; max-width: 64ch; font-size: var(--text-sm); line-height: 1.65; text-wrap: pretty; }
.state-card small { display: block; color: var(--color-text-muted); font-size: var(--text-xs); line-height: 1.6; }
.error-state > .icon { color: var(--color-warning); }
.access-card > .icon { color: var(--color-primary); }
.feedback-actions { display: flex; flex-wrap: wrap; gap: 10px; }
.plan-top { display: grid; grid-template-columns: 1fr 1.05fr; gap: 16px; }
.current-plan { padding: 28px; background: linear-gradient(125deg, var(--color-primary-soft), var(--color-surface) 68%); }
.plan-crown { width: 62px; height: 62px; display: grid; place-items: center; background: linear-gradient(130deg, var(--color-gradient-end), var(--color-primary)); color: var(--color-on-primary); border-radius: 50%; }
.current-plan h2 { font-size: 24px; margin-top: 4px; display: flex; align-items: center; flex-wrap: wrap; gap: 12px; overflow-wrap: anywhere; }
.current-plan > p { font-size: 13px; color: var(--color-text-secondary); margin-top: 18px; }
.plan-dates { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 25px 0; }
.plan-dates span { display: block; color: var(--color-text-muted); font-size: 12px; margin-bottom: 7px; }
.plan-dates strong { font-size: 14px; font-weight: 500; }
.preview-plan { display: block; color: var(--color-text-muted); font-size: var(--text-xs); line-height: 1.5; text-wrap: pretty; margin-top: 18px; }
.quota-overview { padding: 28px; }
.quota-overview > .row-between > span { font-size: 12px; }
.quota-cards { display: grid; grid-template-columns: 1fr 1fr; gap: 27px 24px; margin-top: 26px; }
.quota-item .row-between { font-size: 12px; }
.quota-item .icon { color: var(--color-primary); }
.quota-item strong { display: block; font-size: 22px; margin: 11px 0 12px; }
.quota-item strong small { font-size: 12px; color: var(--color-text-muted); font-weight: 400; }
.plan-section-heading { margin: 25px 0 20px; }
.plan-section-heading > span { font-size: 12px; }
.feature-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 13px 20px; }
.feature-row { display: flex; align-items: center; gap: 14px; border: 1px solid var(--color-border); padding: 17px; border-radius: 9px; }
.feature-row h3 { font-size: 14px; }
.feature-row p { font-size: 12px; color: var(--color-text-muted); margin-top: 5px; line-height: 1.6; }
.feature-icon { height: 40px; width: 40px; border-radius: 12px; display: grid; place-items: center; flex-shrink: 0; background: var(--color-primary-soft); color: var(--color-primary); }
.quota-table { margin-top: 24px; }
.quota-attention { display: grid; gap: 12px; margin-top: 22px; padding: 14px; border-radius: var(--radius-md); background: var(--color-warning-soft); font-size: var(--text-sm); }
.quota-attention p { margin-top: 4px; line-height: 1.6; color: var(--color-text-secondary); }
.expansion-summary { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.expansion-summary span { display: block; color: var(--color-text-muted); font-size: var(--text-xs); }
.expansion-summary strong { display: block; overflow-wrap: anywhere; font-size: var(--text-sm); }
.plan-timeline { margin-top: 28px; }
@media (max-width: 1100px) { .plan-top { grid-template-columns: 1fr; } }
@media (max-width: 767px) {
  .state-card, .access-card { flex-direction: column; padding: 20px; }
  .feature-grid, .quota-cards, .expansion-summary, .plan-dates { grid-template-columns: 1fr; }
  .current-plan, .quota-overview { padding: 20px; }
  .current-plan > .row { flex-wrap: wrap; }
  .feedback-actions .btn { white-space: normal; height: auto; min-height: var(--control-height); }
  .plan-context .btn { margin-inline-start: 0; }
}
</style>
