<script setup lang="ts">
import { computed } from 'vue'
import { backendTermLabel, type BackendTermKind } from '@/i18n/backend-terms'
import { currentUiLocale } from '@/i18n'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { UiButton, UiInput } from '@/ui/base'
import PlanTargetDetails from './PlanTargetDetails.vue'
import PlanChangeReceipt from './PlanChangeReceipt.vue'
import PlanChangeConfirmDialog from './PlanChangeConfirmDialog.vue'
import PlanChangeImpactList from './PlanChangeImpactList.vue'
import PlanChangePreparationImpact from './PlanChangePreparationImpact.vue'
import { usePlanChangeFlow } from '@/features/enterprise/usePlanChangeFlow'
import type { PlanVersionDTO, TenantSubscriptionDTO } from '@/services/commercial/platformCommercial'
import type { TrustedSession } from '@/services/runtime/api'

const props = defineProps<{ session: TrustedSession; subscription: TenantSubscriptionDTO; blocked?: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const {
  opened, action, targets, targetsLoading, selectedTarget, effectiveAt,
  preview, receipt, working, errorMessage, receiptRefreshing, confirmationOpen,
  confirmationUnknown, stage, approvalRequired, canPreview,
  openLifecycle, resetLifecycle, refreshReceipt, createPreview, confirmPreview,
} = usePlanChangeFlow(props, () => emit('changed'))

const actionLabel = computed(() => backendTermLabel('changeAction', action.value))
const currentPlanKey = computed(() => `${backendTermLabel('plan', props.subscription.planCode)} v${props.subscription.planVersion}`)
const previewTargetName = computed(() => action.value === 'SWITCH'
  ? preview.value?.target?.name || backendTermLabel('plan', preview.value?.target?.planCode)
  : backendTermLabel('plan', props.subscription.planCode))
const actionDescription = computed(() => action.value === 'RENEW'
  ? '沿用当前套餐与版本，不需要选择其他套餐；请核对新的权益周期和费用确认条件。'
  : action.value === 'STOP_RENEWAL'
    ? '停止续费不会缩短当前权益期；本操作不提前停用套餐、模块与额度。'
    : '选择目标套餐后，请核对可变更范围、额度影响和生效时间。')
const targetModules = computed(() => (preview.value?.target?.terms?.modules ?? []).map((module) => ({
  id: module.moduleCode,
  label: backendTermLabel('module', module.moduleCode),
  capabilityCount: module.capabilityCodes?.length ?? 0,
  quotaCount: module.quotas?.length ?? 0,
  fieldCount: module.fields?.length ?? 0,
})))
const currentEntitlementSummary = computed(() => {
  const decisions = preview.value?.currentEntitlements?.decisions ?? []
  return {
    modules: decisions.filter((decision) => decision.kind === 'module' && decision.allowed).length,
    capabilities: decisions.filter((decision) => decision.kind === 'capability' && decision.allowed).length,
  }
})
const targetEntitlementSummary = computed(() => ({
  modules: targetModules.value.length,
  features: preview.value?.target?.terms?.featureCodes?.length ?? 0,
  capabilities: targetModules.value.reduce((total, module) => total + module.capabilityCount, 0),
}))
function priceLabel(target: PlanVersionDTO | undefined) {
  if (!target?.terms) return '费用待确认'
  return String(target.terms.priceRef ?? '').trim() ? '费用按销售方案确认' : '免费开通'
}
function validityLabel(target: PlanVersionDTO | undefined) {
  const terms = target?.terms
  if (!terms) return '—'
  if (terms.validityMode === 'fixed_days') return `${terms.validityDays} 天`
  return terms.validityMode === 'unlimited' ? '长期有效' : '—'
}
function planLabel(kind: BackendTermKind, values?: string[]) {
  return values?.length ? values.map((value) => backendTermLabel(kind, value)).join(' · ') : '未声明适用范围'
}
function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
</script>

<template>
  <section class="card lifecycle" data-plan-change-lifecycle>
    <div class="lifecycle-head">
      <div>
        <div class="eyebrow">套餐变更</div>
        <h2>套餐变更</h2>
        <p>{{ actionDescription }}确认后可在此查看处理结果。</p>
      </div>
      <UiButton v-if="!opened" class="btn btn-primary" :disabled="blocked" data-plan-change-open @click="openLifecycle">变更套餐</UiButton>
      <UiButton v-else class="btn" @click="opened = false">收起</UiButton>
    </div>
    <div v-if="opened" class="lifecycle-body">
      <div class="steps" aria-label="套餐变更步骤">
        <span :class="{ active: stage === 'select' }"><b>1</b> 选择变更</span>
        <span :class="{ active: stage === 'preview' }"><b>2</b> 确认方案</span>
        <span :class="{ active: stage === 'receipt' }"><b>3</b> 处理结果</span>
      </div>
      <div v-if="errorMessage" class="change-alert" role="alert"><AppIcon name="help" :size="16" />{{ errorMessage }}</div>
      <div v-if="confirmationUnknown" class="change-alert" role="status" data-plan-change-uncertain>
        <div class="flex-1"><strong>提交结果尚未确认</strong><p>请先检查原操作状态，不要重复提交套餐变更。</p></div>
        <UiButton class="btn" :disabled="receiptRefreshing" data-plan-change-receipt-refresh @click="refreshReceipt">检查处理结果</UiButton>
      </div>
      <template v-if="stage === 'select'">
        <div class="choice-block">
          <span class="field-label">变更方式</span>
          <div class="action-grid">
            <UiButton :class="['action-card', { selected: action === 'SWITCH' }]" :disabled="blocked || working" @click="action = 'SWITCH'"><strong>切换套餐</strong><small>升级或降级到当前可选套餐</small></UiButton>
            <UiButton :class="['action-card', { selected: action === 'RENEW' }]" :disabled="blocked || working" @click="action = 'RENEW'"><strong>续订</strong><small>沿用当前 {{ currentPlanKey }}，不选择其他套餐</small></UiButton>
            <UiButton :class="['action-card', { selected: action === 'STOP_RENEWAL' }]" :disabled="blocked || working" @click="action = 'STOP_RENEWAL'"><strong>停止续费</strong><small>不缩短当前权益期</small></UiButton>
          </div>
          <div v-if="action === 'RENEW'" class="action-guidance"><AppIcon name="calendar" :size="18" /><div><strong>续订当前套餐</strong><p>当前 {{ currentPlanKey }} 为续订对象，无需选择其他套餐。</p></div></div>
          <div v-else-if="action === 'STOP_RENEWAL'" class="action-guidance warning"><AppIcon name="help" :size="18" /><div><strong>当前权益不会提前结束</strong><p>停止续费只改变后续续订意图，不会删除资源或立即停用模块。</p></div></div>
          <PlanTargetDetails v-if="action === 'SWITCH' && selectedTarget" :target="selectedTarget" />
        </div>
        <div v-if="action === 'SWITCH'" class="choice-block">
          <div class="row-between"><span class="field-label">可切换套餐</span><span class="muted">仅显示当前可选择的套餐</span></div>
          <div v-if="targetsLoading" class="inline-state">正在读取可选套餐…</div>
          <div v-else class="target-grid">
            <UiButton v-for="target in targets" :key="`${target.planCode}:${target.version}`" type="button" :class="['target-card', { selected: selectedTarget?.planCode === target.planCode && selectedTarget?.version === target.version }]" :disabled="blocked || working" @click="selectedTarget = target">
              <span class="row-between"><strong>{{ target.name || backendTermLabel('plan', target.planCode) }}</strong><span>v{{ target.version }}</span></span>
              <small>{{ backendTermLabel('plan', target.planCode) }}</small>
              <span class="target-meta">{{ priceLabel(target) }} · {{ validityLabel(target) }}</span>
              <span class="target-facts"><small>{{ target.terms?.modules?.length ?? 0 }} 个模块</small><small>{{ planLabel('salesScope', target.terms?.salesScope) }}</small></span>
            </UiButton>
            <div v-if="targets.length === 0" class="empty-target">当前销售范围没有其他可切换的已发布套餐。</div>
          </div>
        </div>
        <div class="form-grid">
          <label v-if="action !== 'STOP_RENEWAL'" class="form-field"><span class="field-label">{{ action === 'RENEW' ? '续订生效时间' : '计划生效时间' }} <small>可选</small></span><UiInput v-model="effectiveAt" type="datetime-local" :disabled="blocked || working" /></label>
        </div>
        <div class="action-row">
          <span class="boundary-note">变更只针对当前企业，生成方案不会修改当前套餐。</span>
          <UiButton class="btn btn-primary" :disabled="!canPreview" data-plan-change-preview @click="createPreview">{{ working ? '正在生成…' : action === 'RENEW' ? '查看续订方案' : action === 'STOP_RENEWAL' ? '查看停止续费方案' : '查看变更方案' }}</UiButton>
        </div>
      </template>
      <template v-else-if="stage === 'preview' && preview">
        <div class="preview-hero">
          <div>
            <span class="field-label">{{ actionLabel }}</span>
            <h3 v-if="action === 'SWITCH'">{{ backendTermLabel('plan', subscription.planCode) }} → {{ previewTargetName }}</h3>
            <h3 v-else-if="action === 'RENEW'">续订 {{ currentPlanKey }}</h3>
            <h3 v-else>停止 {{ currentPlanKey }} 自动续费</h3>
            <p>{{ backendTermLabel('changeClassification', preview.classification) }} · {{ backendTermLabel('effectiveMode', preview.mode) }} · 请于 {{ formatDate(preview.expiresAt) }} 前确认</p>
          </div>
          <StatusBadge :text="backendTermLabel('changeClassification', preview.classification)" :dot="false" />
        </div>
        <div class="preview-grid">
          <div><span>变更编号</span><strong>{{ preview.changeId }}</strong></div>
          <div><span>生效时间</span><strong>{{ formatDate(preview.effectiveAt) }}</strong></div>
          <div><span>费用确认</span><strong>{{ approvalRequired ? '需要进一步确认' : '无需额外商业确认' }}</strong></div>
          <div><span>额度复核</span><strong>{{ preview.quotaValidationRequired ? '确认前需要复核' : '当前可确认' }}</strong></div>
        </div>
        <section class="comparison-panel" data-plan-comparison>
          <div class="comparison-head">
            <div><h4>{{ action === 'RENEW' ? '续订方案核对' : action === 'STOP_RENEWAL' ? '停止续费影响' : '套餐对比' }}</h4><p>{{ action === 'SWITCH' ? '确认时会再次核对套餐内容和费用条件。' : '请核对当前与处理后的权益期限及保持不变的内容。' }}</p></div>
            <span v-if="action === 'SWITCH'" class="comparison-price">{{ priceLabel(preview.target) }}</span>
          </div>
          <div class="comparison-summary">
            <div><span>当前套餐</span><strong>{{ currentPlanKey }}</strong><small>{{ currentEntitlementSummary.modules }} 个模块 · {{ currentEntitlementSummary.capabilities }} 项能力</small></div>
            <div v-if="action === 'SWITCH'"><span>目标套餐</span><strong>{{ previewTargetName }} · v{{ preview.target?.version }}</strong><small>{{ targetEntitlementSummary.modules }} 个模块 · {{ targetEntitlementSummary.features }} 项商业功能 · {{ targetEntitlementSummary.capabilities }} 项能力</small></div>
            <div v-else><span>当前权益到期日</span><strong>{{ formatDate(preview.before?.periodEnd) }}</strong><small>不提前结束当前权益</small></div>
            <div><span>{{ action === 'RENEW' ? '续订后权益到期日' : action === 'STOP_RENEWAL' ? '处理后权益到期日' : '权益周期' }}</span><strong>{{ action === 'SWITCH' ? validityLabel(preview.target) : formatDate(preview.entitlementExpiresAt) }}</strong></div>
          </div>
          <div v-if="action === 'SWITCH'" class="comparison-modules">
            <article v-for="module in targetModules" :key="module.id" class="comparison-module"><strong>{{ module.label }}</strong><span>{{ module.capabilityCount }} 项能力</span><small>{{ module.quotaCount }} 项额度 · {{ module.fieldCount }} 项字段策略</small></article>
            <p v-if="!targetModules.length" class="muted">目标套餐未返回模块明细，不能继续确认。</p>
          </div>
        </section>
        <div v-if="approvalRequired" class="approval-card" data-plan-change-external-approval><AppIcon name="shield" :size="20" /><div class="flex-1"><strong>需要进一步确认费用</strong><p>费用条件尚未确认，暂不能提交。本次查看方案不会创建支付订单。</p></div></div>
        <PlanChangePreparationImpact :preview="preview" />
        <PlanChangeImpactList :impacts="preview.impactDetails ?? []" :fallback-messages="preview.impacts ?? []" />
        <p class="boundary-note">现有租户数据将被保留，本次操作不会删除资源。</p>
        <div class="action-row">
          <UiButton class="btn" :disabled="working || confirmationUnknown" @click="resetLifecycle">返回重选</UiButton>
          <UiButton class="btn btn-primary" :disabled="blocked || working || confirmationUnknown || approvalRequired || preview.quotaValidationRequired" data-plan-change-confirm @click="confirmationOpen = true">{{ approvalRequired ? '等待外部审批' : working ? '正在确认…' : action === 'RENEW' ? '确认续订' : action === 'STOP_RENEWAL' ? '确认停止续费' : '确认变更' }}</UiButton>
        </div>
      </template>
      <PlanChangeReceipt v-else-if="stage === 'receipt' && receipt" :receipt="receipt" :refreshing="receiptRefreshing" @refresh="refreshReceipt" @reset="resetLifecycle" />
      <PlanChangeConfirmDialog v-if="preview" :open="confirmationOpen" :preview="preview" :pending="working" @close="confirmationOpen = false" @confirm="confirmPreview" />
    </div>
  </section>
</template>

<style scoped>
.lifecycle { padding: 24px; }
.lifecycle-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
.lifecycle-head h2 { margin-top: 4px; font-size: 20px; }
.lifecycle-head p { margin-top: 6px; color: var(--color-text-secondary); font-size: 13px; line-height: 1.6; }
.eyebrow, .field-label { color: var(--color-text-muted); font-size: 12px; font-weight: 600; }
.lifecycle-body { margin-top: 22px; padding-top: 20px; border-top: 1px solid var(--color-border); }
.steps { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-bottom: 20px; }
.steps span { padding: 10px 12px; border-radius: 10px; background: var(--color-surface-soft); color: var(--color-text-muted); font-size: 12px; }
.steps span.active { background: var(--color-primary-soft); color: var(--color-primary); }
.steps b { display: inline-grid; place-items: center; width: 20px; height: 20px; margin-right: 6px; border: 1px solid currentColor; border-radius: 50%; }
.change-alert, .approval-card { display: flex; gap: 10px; align-items: flex-start; padding: 13px 14px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-soft); color: var(--color-text-secondary); font-size: 12px; line-height: 1.6; margin-bottom: 18px; }
.choice-block { margin-top: 18px; }
.action-guidance { display: flex; gap: 12px; align-items: flex-start; margin-top: 12px; padding: 14px; border-radius: 10px; background: var(--color-primary-soft); color: var(--color-primary); }
.action-guidance.warning { background: var(--color-warning-soft); color: var(--color-text-secondary); }
.action-guidance strong { display: block; font-size: var(--text-sm); }
.action-guidance p { margin-top: 4px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.55; }
.action-grid, .target-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 10px; }
.action-card { min-height: 82px; display: flex; flex-direction: column; align-items: flex-start; justify-content: center; gap: 5px; padding: 14px; text-align: left; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.action-card small { color: var(--color-text-muted); white-space: normal; }
.action-card.selected { border-color: var(--color-primary); background: var(--color-primary-soft); color: var(--color-primary); }
.target-card { min-width: 0; padding: 15px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); color: var(--color-text); text-align: left; cursor: pointer; }
.target-card:hover, .target-card.selected { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-soft); }
.target-card small { display: block; margin-top: 6px; color: var(--color-text-muted); }
.target-meta { display: block; margin-top: 12px; color: var(--color-text-secondary); font-size: 12px; }
.target-facts { display: flex; flex-wrap: wrap; gap: 5px 8px; margin-top: 8px; }
.target-facts small { margin: 0; padding: 2px 5px; border-radius: 4px; background: var(--color-surface-soft); }
.empty-target, .inline-state { grid-column: 1 / -1; padding: 22px; border-radius: 10px; background: var(--color-surface-soft); color: var(--color-text-muted); text-align: center; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-top: 20px; }
.form-field { display: grid; gap: 7px; }
.field-label small { font-weight: 400; }
.action-row { display: flex; justify-content: flex-end; align-items: center; gap: 12px; margin-top: 20px; }
.boundary-note { margin-right: auto; max-width: 620px; color: var(--color-text-muted); font-size: var(--text-xs); line-height: 1.6; }
.preview-hero { display: flex; align-items: center; gap: 16px; padding: 18px; border-radius: 12px; background: var(--color-primary-soft); }
.preview-hero > div:first-child { flex: 1; min-width: 0; }
.preview-hero h3 { margin-top: 3px; font-size: 20px; overflow-wrap: anywhere; }
.preview-hero p { margin-top: 5px; color: var(--color-text-secondary); font-size: 12px; }
.preview-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }
.preview-grid div { min-width: 0; padding: 13px; border-radius: 10px; background: var(--color-surface-soft); }
.preview-grid span { display: block; color: var(--color-text-muted); font-size: var(--text-xs); margin-bottom: 5px; }
.preview-grid strong { display: block; font-size: 12px; overflow-wrap: anywhere; }
.comparison-panel { margin-top: 14px; padding: 16px; border: 1px solid var(--color-border); border-radius: 10px; }
.comparison-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.comparison-head h4 { margin: 0; font-size: 13px; }
.comparison-head p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.5; }
.comparison-price { flex: 0 0 auto; font-weight: 700; color: var(--color-primary); }
.comparison-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: 14px; }
.comparison-summary > div { min-width: 0; padding: 10px; border-radius: 8px; background: var(--color-surface-soft); }
.comparison-summary span, .comparison-module span, .comparison-module small { display: block; color: var(--color-text-muted); font-size: var(--text-xs); }
.comparison-summary strong { display: block; margin-top: 4px; font-size: 12px; overflow-wrap: anywhere; }
.comparison-summary small { display: block; margin-top: 4px; color: var(--color-text-muted); font-size: var(--text-xs); }
.comparison-modules { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: 12px; }
.comparison-module { min-width: 0; padding: 11px; border: 1px solid var(--color-border); border-radius: 8px; }
.comparison-module strong { display: block; font-size: 12px; overflow-wrap: anywhere; }
.comparison-module span { margin-top: 6px; }
.comparison-module small { margin-top: 3px; }
.approval-card { margin-top: 14px; background: var(--color-warning-soft); }
.approval-card strong { color: var(--color-text); }
.approval-card p { margin-top: 3px; }
.muted { color: var(--color-text-muted); font-size: 12px; }
@media (max-width: 900px) {
  .lifecycle-head { flex-direction: column; }
  .action-grid, .target-grid, .preview-grid { grid-template-columns: 1fr 1fr; }
  .comparison-summary, .comparison-modules, .form-grid { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .action-grid, .target-grid, .preview-grid, .steps { grid-template-columns: 1fr; }
  .action-row { align-items: stretch; flex-direction: column; }
  .action-row .btn { width: 100%; }
  .change-alert { flex-wrap: wrap; }
}
</style>
