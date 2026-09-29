<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { backendBusinessText, backendTermLabel, type BackendTermKind } from '@/i18n/backend-terms'
import { currentUiLocale } from '@/i18n'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { UiButton, UiInput } from '@/ui/base'
import PlanTargetDetails from '@/features/enterprise/components/PlanTargetDetails.vue'
import PlanChangeReceipt from '@/features/enterprise/components/PlanChangeReceipt.vue'
import PlanChangeConfirmDialog from '@/features/enterprise/components/PlanChangeConfirmDialog.vue'
import type {
  PlanVersionDTO,
  PaymentOrderDTO,
  SubscriptionChangePreviewDTO,
  SubscriptionChangeReceiptDTO,
  TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'
import type { TrustedSession } from '@/services/runtime/api'
import {
  confirmMySubscriptionChange,
  createMyPaymentOrder,
  getMySubscriptionChangeReceipt,
  listMySubscriptionChangeTargets,
  needsExternalCommercialApproval,
  previewMySubscriptionChange,
  tenantChangeRuntimeError,
  type TenantChangeAction,
  type TenantPaymentProvider,
} from '@/services/enterprise/planChangeRuntime'

const props = defineProps<{
  session: TrustedSession
  subscription: TenantSubscriptionDTO
}>()
const emit = defineEmits<{ changed: [] }>()

const opened = ref(false)
const action = ref<TenantChangeAction>('SWITCH')
const targets = ref<PlanVersionDTO[]>([])
const targetsLoading = ref(false)
const selectedTarget = ref<PlanVersionDTO | null>(null)
const reason = ref('租户自服务套餐变更')
const effectiveAt = ref('')
const preview = ref<SubscriptionChangePreviewDTO | null>(null)
const receipt = ref<SubscriptionChangeReceiptDTO | null>(null)
const paymentOrder = ref<PaymentOrderDTO | null>(null)
const working = ref(false)
const errorMessage = ref('')
const receiptRefreshing = ref(false)
const confirmationOpen = ref(false)
let receiptTimer: ReturnType<typeof setInterval> | undefined

const actionLabel = computed(() => backendTermLabel('changeAction', action.value))
const stage = computed(() => receipt.value ? 'receipt' : preview.value ? 'preview' : 'select')
const requiresTarget = computed(() => action.value === 'SWITCH')
const approvalRequired = computed(() => needsExternalCommercialApproval(preview.value))
const canPreview = computed(() => {
  if (!reason.value.trim()) return false
  if (requiresTarget.value && !selectedTarget.value) return false
  return true
})
const currentPlanKey = computed(() => `${backendTermLabel('plan', props.subscription.planCode)} v${props.subscription.planVersion}`)
const previewTargetName = computed(() => preview.value?.target?.name || backendTermLabel('plan', preview.value?.target?.planCode))
const targetModules = computed(() => (preview.value?.target?.terms?.modules ?? []).map((module) => ({
  id: module.moduleCode,
  label: moduleLabel(module.moduleCode),
  capabilityCount: module.capabilityCodes?.length ?? 0,
  quotaCount: module.quotas?.length ?? 0,
  fieldCount: module.fields?.length ?? 0,
})))
const currentEntitlementSummary = computed(() => {
  const decisions = preview.value?.currentEntitlements?.decisions ?? []
  return {
    modules: decisions.filter((decision) => decision.kind === 'module' && decision.allowed).length,
    capabilities: decisions.filter((decision) => decision.kind === 'capability' && decision.allowed).length,
    quotas: decisions.filter((decision) => decision.kind === 'quota' && decision.allowed).length,
    fields: decisions.filter((decision) => decision.kind === 'field' && decision.allowed).length,
  }
})
const targetEntitlementSummary = computed(() => ({
  modules: targetModules.value.length,
  capabilities: targetModules.value.reduce((total, module) => total + module.capabilityCount, 0),
  quotas: targetModules.value.reduce((total, module) => total + module.quotaCount, 0),
  fields: targetModules.value.reduce((total, module) => total + module.fieldCount, 0),
}))

watch(action, () => {
  selectedTarget.value = null
  preview.value = null
  receipt.value = null
  stopReceiptSync()
  paymentOrder.value = null
  errorMessage.value = ''
  if (action.value === 'STOP_RENEWAL') effectiveAt.value = ''
})

function priceLabel(target: PlanVersionDTO | undefined) {
  const terms = target?.terms
  if (!String(terms?.priceRef ?? '').trim()) return '免费开通'
  const amount = Number(terms?.amountMinor)
  const currency = String(terms?.currency ?? '')
  if (!Number.isSafeInteger(amount) || amount < 1 || !/^[A-Z]{3}$/.test(currency)) return '价格待确认'
  return new Intl.NumberFormat(currentUiLocale(), { style: 'currency', currency }).format(amount / 100)
}

function validityLabel(target: PlanVersionDTO | undefined) {
  const terms = target?.terms
  if (!terms) return '—'
  if (terms.validityMode === 'fixed_days') return `${terms.validityDays} 天`
  return terms.validityMode === 'unlimited' ? '长期有效' : terms.validityMode || '—'
}

function moduleLabel(code: string) { return backendTermLabel('module', code) }
function planLabel(kind: BackendTermKind, values?: string[]) {
  return values?.length ? values.map((value) => backendTermLabel(kind, value)).join(' · ') : '未声明适用范围'
}

async function createPaymentOrder(provider: TenantPaymentProvider) {
  if (!preview.value) return
  working.value = true
  errorMessage.value = ''
  try {
    paymentOrder.value = await createMyPaymentOrder(props.session, preview.value, provider)
  } catch (error) {
    paymentOrder.value = null
    errorMessage.value = tenantChangeRuntimeError(error)
  } finally {
    working.value = false
  }
}

function classificationLabel(value?: string) {
  return backendTermLabel('changeClassification', value)
}

function effectiveModeLabel(value?: string) {
  return backendTermLabel('effectiveMode', value)
}

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(currentUiLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function effectiveAtIso() {
  if (!effectiveAt.value || action.value === 'STOP_RENEWAL') return ''
  const value = new Date(effectiveAt.value)
  return Number.isNaN(value.getTime()) ? '' : value.toISOString()
}

async function openLifecycle() {
  opened.value = true
  errorMessage.value = ''
  if (targets.value.length > 0) return
  targetsLoading.value = true
  try {
    const result = await listMySubscriptionChangeTargets(props.session)
    targets.value = Array.isArray(result.targets) ? result.targets : []
  } catch (error) {
    errorMessage.value = tenantChangeRuntimeError(error)
  } finally {
    targetsLoading.value = false
  }
}

function resetLifecycle() {
  stopReceiptSync()
  confirmationOpen.value = false
  preview.value = null
  receipt.value = null
  errorMessage.value = ''
}

function receiptNeedsSync(value: SubscriptionChangeReceiptDTO | null) { return value?.status === 'SCHEDULED' || value?.status === 'PROVISIONING' }
function stopReceiptSync() { if (receiptTimer) clearInterval(receiptTimer); receiptTimer = undefined }
function startReceiptSync() { stopReceiptSync(); if (receiptNeedsSync(receipt.value)) receiptTimer = setInterval(() => { void refreshReceipt() }, 15_000) }
async function refreshReceipt() {
  if (!receipt.value || receiptRefreshing.value) return
  receiptRefreshing.value = true
  errorMessage.value = ''
  try {
    receipt.value = await getMySubscriptionChangeReceipt(props.session, receipt.value.changeId)
    if (!receiptNeedsSync(receipt.value)) { stopReceiptSync(); emit('changed') }
  } catch (error) {
    errorMessage.value = tenantChangeRuntimeError(error)
  } finally {
    receiptRefreshing.value = false
  }
}
onBeforeUnmount(stopReceiptSync)

async function createPreview() {
  if (!canPreview.value) return
  working.value = true
  errorMessage.value = ''
  receipt.value = null
  try {
    preview.value = await previewMySubscriptionChange(props.session, {
      action: action.value,
      targetPlanCode: selectedTarget.value?.planCode,
      targetPlanVersion: selectedTarget.value?.version,
      effectiveAt: effectiveAtIso(),
      reason: reason.value,
    })
  } catch (error) {
    preview.value = null
    errorMessage.value = tenantChangeRuntimeError(error)
  } finally {
    working.value = false
  }
}

async function confirmPreview() {
  if (!preview.value || approvalRequired.value) return
  working.value = true
  errorMessage.value = ''
  try {
    confirmationOpen.value = false
    receipt.value = await confirmMySubscriptionChange(props.session, preview.value, reason.value)
    if (receiptNeedsSync(receipt.value)) startReceiptSync()
    else emit('changed')
  } catch (error) {
    errorMessage.value = tenantChangeRuntimeError(error)
  } finally {
    working.value = false
  }
}
</script>

<template>
  <section class="card lifecycle" data-plan-change-lifecycle>
    <div class="lifecycle-head">
      <div>
        <div class="eyebrow">套餐变更</div>
        <h2>套餐变更</h2>
        <p>选择目标套餐后，系统会核对可变更范围、额度影响和生效时间，并在确认后展示处理结果。</p>
      </div>
      <UiButton v-if="!opened" class="btn primary" data-plan-change-open @click="openLifecycle">
        变更套餐
      </UiButton>
      <UiButton v-else class="btn" @click="opened = false">收起</UiButton>
    </div>

    <div v-if="opened" class="lifecycle-body">
      <div class="steps" aria-label="套餐变更步骤">
        <span :class="{ active: stage === 'select' }"><b>1</b> 选择变更</span>
        <span :class="{ active: stage === 'preview' }"><b>2</b> 确认方案</span>
        <span :class="{ active: stage === 'receipt' }"><b>3</b> 处理结果</span>
      </div>

      <div v-if="errorMessage" class="change-alert" role="alert">
        <AppIcon name="help" :size="16" /> {{ errorMessage }}
      </div>

      <template v-if="stage === 'select'">
        <div class="choice-block">
          <span class="field-label">变更方式</span>
          <div class="action-grid">
            <UiButton :class="['action-card', { selected: action === 'SWITCH' }]" @click="action = 'SWITCH'">
              <strong>切换套餐</strong><small>升级或降级到当前可选套餐</small>
            </UiButton>
            <UiButton :class="['action-card', { selected: action === 'RENEW' }]" @click="action = 'RENEW'">
              <strong>续订</strong><small>续订当前 {{ currentPlanKey }}</small>
            </UiButton>
            <UiButton :class="['action-card', { selected: action === 'STOP_RENEWAL' }]" @click="action = 'STOP_RENEWAL'">
              <strong>停止续费</strong><small>仅停止续费意图，不缩短当前权益期</small>
            </UiButton>
          </div>
        </div>

        <div v-if="action === 'SWITCH'" class="choice-block">
          <div class="row-between">
            <span class="field-label">可切换套餐</span>
            <span class="muted">仅显示当前可选择的套餐</span>
          </div>
          <div v-if="targetsLoading" class="inline-state">正在读取可选套餐…</div>
          <div v-else class="target-grid">
            <UiButton
              v-for="target in targets"
              :key="`${target.planCode}:${target.version}`"
              type="button"
              :class="['target-card', { selected: selectedTarget?.planCode === target.planCode && selectedTarget?.version === target.version }]"
              @click="selectedTarget = target"
            >
              <span class="row-between"><strong>{{ target.name || backendTermLabel('plan', target.planCode) }}</strong><span>v{{ target.version }}</span></span>
              <small>{{ backendTermLabel('plan', target.planCode) }}</small>
              <span class="target-meta">{{ priceLabel(target) }} · {{ validityLabel(target) }}</span>
              <span class="target-facts">
                <small>{{ target.terms?.modules?.length ?? 0 }} 个模块</small>
                <small>{{ planLabel('salesScope', target.terms?.salesScope) }}</small>
              </span>
            </UiButton>
            <div v-if="targets.length === 0" class="empty-target">当前销售范围没有其他可切换的已发布套餐。</div>
          </div>
          <PlanTargetDetails v-if="selectedTarget" :target="selectedTarget" />
        </div>

        <div class="form-grid">
          <label class="form-field">
            <span class="field-label">变更原因</span>
            <UiInput v-model="reason" placeholder="请输入本次变更原因" />
          </label>
          <label v-if="action !== 'STOP_RENEWAL'" class="form-field">
            <span class="field-label">计划生效时间 <small>可选</small></span>
            <UiInput v-model="effectiveAt" type="datetime-local" />
          </label>
        </div>

        <div class="action-row">
          <span class="boundary-note">变更只针对当前企业，已有用量和已生效权益不会被页面直接改写。</span>
          <UiButton class="btn primary" :disabled="!canPreview || working" data-plan-change-preview @click="createPreview">
            {{ working ? '正在生成…' : '查看变更方案' }}
          </UiButton>
        </div>
      </template>

      <template v-else-if="stage === 'preview' && preview">
        <div class="preview-hero">
          <div>
            <span class="field-label">{{ actionLabel }}</span>
            <h3>{{ backendTermLabel('plan', subscription.planCode) }} → {{ previewTargetName }}</h3>
            <p>{{ classificationLabel(preview.classification) }} · {{ effectiveModeLabel(preview.mode) }} · 请于 {{ formatDate(preview.expiresAt) }} 前确认</p>
          </div>
          <StatusBadge :text="classificationLabel(preview.classification)" :dot="false" />
        </div>

        <div class="preview-grid">
          <div><span>变更编号</span><strong>{{ preview.changeId }}</strong></div>
          <div><span>生效时间</span><strong>{{ formatDate(preview.effectiveAt) }}</strong></div>
          <div><span>费用确认</span><strong>{{ approvalRequired ? '需要进一步确认' : '无需额外确认' }}</strong></div>
          <div><span>额度复核</span><strong>{{ preview.quotaValidationRequired ? '确认前需要复核' : '当前可确认' }}</strong></div>
        </div>

        <section class="comparison-panel" data-plan-comparison>
          <div class="comparison-head">
            <div><h4>套餐对比</h4><p>套餐内容与价格均以本次预览为准，确认时会再次核对。</p></div>
            <span class="comparison-price">{{ priceLabel(preview.target) }}</span>
          </div>
          <div class="comparison-summary">
            <div><span>当前套餐</span><strong>{{ currentPlanKey }}</strong><small>{{ currentEntitlementSummary.modules }} 个模块 · {{ currentEntitlementSummary.capabilities }} 项能力</small></div>
            <div><span>目标套餐</span><strong>{{ previewTargetName }} · v{{ preview.target?.version }}</strong><small>{{ targetEntitlementSummary.modules }} 个模块 · {{ targetEntitlementSummary.capabilities }} 项能力</small></div>
            <div><span>权益周期</span><strong>{{ validityLabel(preview.target) }}</strong></div>
          </div>
          <div class="comparison-modules">
            <article v-for="module in targetModules" :key="module.id" class="comparison-module">
              <strong>{{ module.label }}</strong>
              <span>{{ module.capabilityCount }} 项能力</span>
              <small>{{ module.quotaCount }} 项额度 · {{ module.fieldCount }} 项字段策略</small>
            </article>
            <p v-if="!targetModules.length" class="muted">目标套餐未返回模块明细，不能继续确认。</p>
          </div>
        </section>

        <div v-if="approvalRequired" class="approval-card" data-plan-change-external-approval>
          <AppIcon name="shield" :size="20" />
          <div class="flex-1"><strong>选择支付方式并生成订单</strong><p>系统会以当前预览锁定的套餐版本、币种与金额创建订单；支付成功前不会开通新权益。</p>
            <div v-if="!paymentOrder" class="payment-actions">
              <UiButton class="btn" :disabled="working" @click="createPaymentOrder('WECHAT_NATIVE')">微信扫码支付</UiButton>
              <UiButton class="btn" :disabled="working" @click="createPaymentOrder('ALIPAY_PAGE')">支付宝支付</UiButton>
            </div>
            <div v-else class="payment-order" data-payment-order>
              <strong>订单已创建：{{ paymentOrder.orderId }}</strong>
              <span>{{ paymentOrder.provider === 'WECHAT_NATIVE' ? '微信扫码' : '支付宝网页' }} · {{ paymentOrder.currency }} {{ Number(paymentOrder.amountMinor) / 100 }}</span>
              <small>订单状态：{{ backendTermLabel('paymentState', paymentOrder.state) }}。完成支付核验后才可进入开通确认；本页不会把创建订单视为已支付或已开通。</small>
            </div>
          </div>
        </div>

        <div class="impact-grid">
          <div class="impact-card">
            <h4>额度影响</h4>
            <div v-if="preview.quotaImpacts?.length" class="impact-list">
              <div v-for="(item, index) in preview.quotaImpacts" :key="`${item.moduleCode}:${item.key}`">
                <span>{{ backendTermLabel('entitlementKey', item.key) }} · {{ index + 1 }}</span>
                <strong>{{ item.usageKnown ? `已用 ${item.used}` : '用量未知' }}</strong>
                <small>{{ item.overLimit ? '超过目标额度' : item.policy || '通过当前校验' }}</small>
              </div>
            </div>
            <p v-else class="muted">本次变更没有额度差异。</p>
          </div>
          <div class="impact-card">
            <h4>准备与依赖</h4>
            <p v-if="preview.provisioningRequirements?.length">需要完成 {{ preview.provisioningRequirements.length }} 项准备工作后再生效。</p>
            <p v-else>无需额外准备，可按当前方案生效。</p>
            <p v-if="preview.dependencies?.length" class="muted">涉及 {{ preview.dependencies.length }} 个模块依赖。</p>
          </div>
        </div>

        <div class="impact-card impacts">
          <h4>变更影响</h4>
          <ul><li v-for="impact in preview.impacts" :key="impact">{{ backendBusinessText(impact) }}</li></ul>
        </div>

        <div class="action-row">
          <UiButton class="btn" @click="resetLifecycle">返回重选</UiButton>
          <UiButton
            class="btn primary"
            :disabled="working || approvalRequired || preview.quotaValidationRequired"
            data-plan-change-confirm
            @click="confirmationOpen = true"
          >
            {{ approvalRequired ? '等待外部审批' : working ? '正在确认…' : '确认变更' }}
          </UiButton>
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
.steps span { padding: 10px 12px; border-radius: 10px; background: var(--color-surface-muted); color: var(--color-text-muted); font-size: 12px; }
.steps span.active { background: var(--color-primary-soft); color: var(--color-primary); }
.steps b { display: inline-grid; place-items: center; width: 20px; height: 20px; margin-right: 6px; border: 1px solid currentColor; border-radius: 50%; }
.change-alert, .approval-card, .result-note { display: flex; gap: 10px; align-items: flex-start; padding: 13px 14px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-muted); color: var(--color-text-secondary); font-size: 12px; line-height: 1.6; margin-bottom: 18px; }
.change-alert { border-color: var(--color-warning-border, var(--color-border)); }
.choice-block { margin-top: 18px; }
.action-grid, .target-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 10px; }
.action-card { min-height: 82px; display: flex; flex-direction: column; align-items: flex-start; justify-content: center; gap: 5px; padding: 14px; text-align: left; }
.action-card small { color: var(--color-text-muted); white-space: normal; }
.action-card.selected { border-color: var(--color-primary); background: var(--color-primary-soft); color: var(--color-primary); }
.target-card { min-width: 0; padding: 15px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); color: var(--color-text-primary); text-align: left; cursor: pointer; }
.target-card:hover, .target-card.selected { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-soft); }
.target-card small { display: block; margin-top: 6px; color: var(--color-text-muted); }
.target-meta { display: block; margin-top: 12px; color: var(--color-text-secondary); font-size: 12px; }
.target-facts { display: flex; flex-wrap: wrap; gap: 5px 8px; margin-top: 8px; }
.target-facts small { margin: 0; padding: 2px 5px; border-radius: 4px; background: var(--color-surface-muted); color: var(--color-text-secondary); }
.empty-target, .inline-state { grid-column: 1 / -1; padding: 22px; border-radius: 10px; background: var(--color-surface-muted); color: var(--color-text-muted); text-align: center; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-top: 20px; }
.form-field { display: grid; gap: 7px; }
.field-label small { font-weight: 400; }
.action-row { display: flex; justify-content: flex-end; align-items: center; gap: 12px; margin-top: 20px; }
.boundary-note { margin-right: auto; max-width: 620px; color: var(--color-text-muted); font-size: 11px; }
.preview-hero, .result-hero { display: flex; align-items: center; gap: 16px; padding: 18px; border-radius: 12px; background: var(--color-primary-soft); }
.preview-hero > div:first-child, .result-hero > div { flex: 1; min-width: 0; }
.preview-hero h3, .result-hero h3 { margin-top: 3px; font-size: 20px; overflow-wrap: anywhere; }
.preview-hero p, .result-hero p { margin-top: 5px; color: var(--color-text-secondary); font-size: 12px; }
.preview-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }
.preview-grid div { min-width: 0; padding: 13px; border-radius: 10px; background: var(--color-surface-muted); }
.preview-grid span { display: block; color: var(--color-text-muted); font-size: 11px; margin-bottom: 5px; }
.preview-grid strong { display: block; font-size: 12px; overflow-wrap: anywhere; }
.comparison-panel { margin-top: 14px; padding: 16px; border: 1px solid var(--color-border); border-radius: 10px; }
.comparison-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.comparison-head h4 { margin: 0; font-size: 13px; }
.comparison-head p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.5; }
.comparison-price { flex: 0 0 auto; font-weight: 700; color: var(--color-primary); }
.comparison-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: 14px; }
.comparison-summary > div { min-width: 0; padding: 10px; border-radius: 8px; background: var(--color-surface-muted); }
.comparison-summary span, .comparison-module span, .comparison-module small { display: block; color: var(--color-text-muted); font-size: 11px; }
.comparison-summary strong { display: block; margin-top: 4px; font-size: 12px; overflow-wrap: anywhere; }
.comparison-summary small { display: block; margin-top: 4px; color: var(--color-text-muted); font-size: 11px; }
.comparison-modules { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: 12px; }
.comparison-module { min-width: 0; padding: 11px; border: 1px solid var(--color-border); border-radius: 8px; }
.comparison-module strong { display: block; font-size: 12px; overflow-wrap: anywhere; }
.comparison-module span { margin-top: 6px; }
.comparison-module small { margin-top: 3px; }
.approval-card { margin-top: 14px; background: var(--color-warning-soft, var(--color-surface-muted)); }
.approval-card strong { color: var(--color-text-primary); }
.approval-card p { margin-top: 3px; }
.payment-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.payment-order { display: grid; gap: 4px; margin-top: 12px; padding: 10px; border-radius: 8px; background: var(--color-surface); font-size: 12px; }
.payment-order small { color: var(--color-text-muted); }
.impact-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-top: 14px; }
.impact-card { padding: 16px; border: 1px solid var(--color-border); border-radius: 10px; }
.impact-card h4 { margin-bottom: 10px; font-size: 13px; }
.impact-card p, .impact-card li { color: var(--color-text-secondary); font-size: 12px; line-height: 1.6; }
.impact-list { display: grid; gap: 8px; }
.impact-list > div { display: grid; grid-template-columns: 1fr auto; gap: 2px 12px; padding-bottom: 8px; border-bottom: 1px solid var(--color-border); }
.impact-list small { grid-column: 1 / -1; color: var(--color-text-muted); }
.impacts { margin-top: 14px; }
.impacts ul { margin: 0; padding-left: 18px; }
.result-icon { width: 48px; height: 48px; display: grid; place-items: center; border-radius: 50%; background: var(--color-primary); color: var(--color-on-primary); }
.result-note { margin-top: 14px; margin-bottom: 0; flex-direction: column; gap: 3px; }
.muted { color: var(--color-text-muted); font-size: 12px; }
@media (max-width: 900px) {
  .lifecycle-head { flex-direction: column; }
  .action-grid, .target-grid, .preview-grid { grid-template-columns: 1fr 1fr; }
  .comparison-summary, .comparison-modules { grid-template-columns: 1fr; }
  .impact-grid, .form-grid { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .action-grid, .target-grid, .preview-grid, .steps { grid-template-columns: 1fr; }
  .action-row { align-items: stretch; flex-direction: column; }
  .action-row .btn { width: 100%; }
}
</style>
