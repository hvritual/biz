<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { UiButton, UiInput } from '@/ui/base'
import type {
  PlanVersionDTO,
  SubscriptionChangePreviewDTO,
  SubscriptionChangeReceiptDTO,
  TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'
import type { TrustedSession } from '@/services/runtime/api'
import {
  confirmMySubscriptionChange,
  listMySubscriptionChangeTargets,
  needsExternalCommercialApproval,
  previewMySubscriptionChange,
  tenantChangeRuntimeError,
  type TenantChangeAction,
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
const working = ref(false)
const errorMessage = ref('')

const actionLabel = computed(() => ({
  SWITCH: '切换套餐',
  RENEW: '续订当前套餐',
  STOP_RENEWAL: '停止自动续费',
}[action.value]))
const stage = computed(() => receipt.value ? 'receipt' : preview.value ? 'preview' : 'select')
const requiresTarget = computed(() => action.value === 'SWITCH')
const approvalRequired = computed(() => needsExternalCommercialApproval(preview.value))
const canPreview = computed(() => {
  if (!reason.value.trim()) return false
  if (requiresTarget.value && !selectedTarget.value) return false
  return true
})
const currentPlanKey = computed(() => `${props.subscription.planCode}@${props.subscription.planVersion}`)
const previewTargetName = computed(() => preview.value?.target?.name || preview.value?.target?.planCode || '当前套餐')
const receiptTone = computed(() => {
  if (receipt.value?.status === 'APPLIED') return 'success'
  if (receipt.value?.status === 'SCHEDULED' || receipt.value?.status === 'PROVISIONING') return 'warning'
  return 'neutral'
})

watch(action, () => {
  selectedTarget.value = null
  preview.value = null
  receipt.value = null
  errorMessage.value = ''
  if (action.value === 'STOP_RENEWAL') effectiveAt.value = ''
})

function priceLabel(target: PlanVersionDTO) {
  const ref = String(target.terms?.priceRef ?? '').trim()
  return ref ? `价格引用 ${ref}` : '无外部价格引用'
}

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
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
  preview.value = null
  receipt.value = null
  errorMessage.value = ''
}

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
    receipt.value = await confirmMySubscriptionChange(props.session, preview.value, reason.value)
    emit('changed')
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
        <h2>租户自服务变更生命周期</h2>
        <p>目标发现、权威预览、额度与依赖校验、确认、生效状态和回执均由服务端控制。</p>
      </div>
      <UiButton v-if="!opened" class="btn primary" data-plan-change-open @click="openLifecycle">
        变更套餐
      </UiButton>
      <UiButton v-else class="btn" @click="opened = false">收起</UiButton>
    </div>

    <div v-if="opened" class="lifecycle-body">
      <div class="steps" aria-label="套餐变更步骤">
        <span :class="{ active: stage === 'select' }"><b>1</b> 选择变更</span>
        <span :class="{ active: stage === 'preview' }"><b>2</b> 权威预览</span>
        <span :class="{ active: stage === 'receipt' }"><b>3</b> 结果回执</span>
      </div>

      <div v-if="errorMessage" class="change-alert" role="alert">
        <AppIcon name="help" :size="16" /> {{ errorMessage }}
      </div>

      <template v-if="stage === 'select'">
        <div class="choice-block">
          <span class="field-label">变更方式</span>
          <div class="action-grid">
            <UiButton :class="['action-card', { selected: action === 'SWITCH' }]" @click="action = 'SWITCH'">
              <strong>切换套餐</strong><small>升级或降级到服务端可用版本</small>
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
            <span class="muted">只显示 published + eligible</span>
          </div>
          <div v-if="targetsLoading" class="inline-state">正在读取服务端可用套餐…</div>
          <div v-else class="target-grid">
            <button
              v-for="target in targets"
              :key="`${target.planCode}:${target.version}`"
              type="button"
              :class="['target-card', { selected: selectedTarget?.planCode === target.planCode && selectedTarget?.version === target.version }]"
              @click="selectedTarget = target"
            >
              <span class="row-between"><strong>{{ target.name || target.planCode }}</strong><span>v{{ target.version }}</span></span>
              <small>{{ target.planCode }}</small>
              <span class="target-meta">{{ target.terms?.modules?.length ?? 0 }} 个模块 · {{ priceLabel(target) }}</span>
            </button>
            <div v-if="targets.length === 0" class="empty-target">当前销售范围没有其他可切换的已发布套餐。</div>
          </div>
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
          <span class="boundary-note">浏览器不会提交 tenant_id、sales_scope、用量、支付状态或权益版本。</span>
          <UiButton class="btn primary" :disabled="!canPreview || working" data-plan-change-preview @click="createPreview">
            {{ working ? '正在生成…' : '生成权威预览' }}
          </UiButton>
        </div>
      </template>

      <template v-else-if="stage === 'preview' && preview">
        <div class="preview-hero">
          <div>
            <span class="field-label">{{ actionLabel }}</span>
            <h3>{{ subscription.planCode }} → {{ previewTargetName }}</h3>
            <p>{{ preview.classification }} · {{ preview.mode }} · 预览有效至 {{ formatDate(preview.expiresAt) }}</p>
          </div>
          <StatusBadge :text="preview.classification || 'PREVIEW'" :dot="false" />
        </div>

        <div class="preview-grid">
          <div><span>变更 ID</span><strong>{{ preview.changeId }}</strong></div>
          <div><span>生效时间</span><strong>{{ formatDate(preview.effectiveAt) }}</strong></div>
          <div><span>价格权威</span><strong>{{ preview.pricingBasis || 'NO_PRICE_REFERENCE' }}</strong></div>
          <div><span>额度复核</span><strong>{{ preview.quotaValidationRequired ? '确认前需要复核' : '当前可确认' }}</strong></div>
        </div>

        <div v-if="approvalRequired" class="approval-card" data-plan-change-external-approval>
          <AppIcon name="shield" :size="20" />
          <div><strong>需要外部商业 / 支付审批</strong><p>目标套餐存在价格引用。本页面不会把“点击确认”当作支付凭证，也不会绕过平台商业审批。</p></div>
        </div>

        <div class="impact-grid">
          <div class="impact-card">
            <h4>额度影响</h4>
            <div v-if="preview.quotaImpacts?.length" class="impact-list">
              <div v-for="item in preview.quotaImpacts" :key="`${item.moduleCode}:${item.key}`">
                <span>{{ item.moduleCode }} / {{ item.key }}</span>
                <strong>{{ item.usageKnown ? `已用 ${item.used}` : '用量未知' }}</strong>
                <small>{{ item.overLimit ? '超过目标额度' : item.policy || '通过当前校验' }}</small>
              </div>
            </div>
            <p v-else class="muted">本次变更没有额度差异。</p>
          </div>
          <div class="impact-card">
            <h4>准备与依赖</h4>
            <p v-if="preview.provisioningRequirements?.length">需要 {{ preview.provisioningRequirements.length }} 项外部准备；确认后进入 PROVISIONING。</p>
            <p v-else>无需外部准备，可按预览的生效模式执行。</p>
            <p v-if="preview.dependencies?.length" class="muted">涉及 {{ preview.dependencies.length }} 个模块依赖。</p>
          </div>
        </div>

        <div class="impact-card impacts">
          <h4>服务端影响说明</h4>
          <ul><li v-for="impact in preview.impacts" :key="impact">{{ impact }}</li></ul>
        </div>

        <div class="action-row">
          <UiButton class="btn" @click="resetLifecycle">返回重选</UiButton>
          <UiButton
            class="btn primary"
            :disabled="working || approvalRequired || preview.quotaValidationRequired"
            data-plan-change-confirm
            @click="confirmPreview"
          >
            {{ approvalRequired ? '等待外部审批' : working ? '正在确认…' : '确认变更' }}
          </UiButton>
        </div>
      </template>

      <template v-else-if="stage === 'receipt' && receipt">
        <div class="result-hero" data-plan-change-receipt>
          <span class="result-icon"><AppIcon name="check" :size="28" /></span>
          <div>
            <span class="field-label">服务端变更回执</span>
            <h3>{{ receipt.status }}</h3>
            <p>确认时间 {{ formatDate(receipt.confirmedAt) }} · 生效时间 {{ formatDate(receipt.effectiveAt) }}</p>
          </div>
          <StatusBadge :text="receipt.status" :tone="receiptTone" :dot="false" />
        </div>
        <div class="preview-grid">
          <div><span>变更 ID</span><strong>{{ receipt.changeId }}</strong></div>
          <div><span>执行模式</span><strong>{{ receipt.mode }}</strong></div>
          <div><span>价格授权</span><strong>{{ receipt.pricingAuthority }}</strong></div>
          <div><span>Provisioning</span><strong>{{ receipt.provisioningTaskId || '无需外部准备' }}</strong></div>
        </div>
        <div class="result-note">
          <strong v-if="receipt.status === 'APPLIED'">套餐变更已生效。</strong>
          <strong v-else-if="receipt.status === 'SCHEDULED'">套餐变更已预约。</strong>
          <strong v-else-if="receipt.status === 'PROVISIONING'">套餐变更正在外部准备。</strong>
          <span>页面会重新读取当前订阅；SCHEDULED / PROVISIONING 状态通过 pendingChangeId 持续保持权威关联。</span>
        </div>
        <div class="action-row"><UiButton class="btn" @click="resetLifecycle">发起其他变更</UiButton></div>
      </template>
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
.approval-card { margin-top: 14px; background: var(--color-warning-soft, var(--color-surface-muted)); }
.approval-card strong { color: var(--color-text-primary); }
.approval-card p { margin-top: 3px; }
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
  .impact-grid, .form-grid { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .action-grid, .target-grid, .preview-grid, .steps { grid-template-columns: 1fr; }
  .action-row { align-items: stretch; flex-direction: column; }
  .action-row .btn { width: 100%; }
}
</style>
