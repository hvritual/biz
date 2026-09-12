<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import {
  CommercialApiError,
  commercialRequestId,
  confirmSubscriptionChange,
  previewSubscriptionChange,
  type SubscriptionChangeAction,
  type SubscriptionChangePreviewDTO,
  type SubscriptionChangeReceiptDTO,
  type TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'

const props = defineProps<{
  tenantId: string
  subscription: TenantSubscriptionDTO | null
}>()

const emit = defineEmits<{ refresh: [] }>()

const action = ref<SubscriptionChangeAction>('SWITCH')
const targetPlanCode = ref('')
const targetPlanVersion = ref('')
const effectiveAt = ref('')
const previewReason = ref('')
const preview = ref<SubscriptionChangePreviewDTO | null>(null)
const receipt = ref<SubscriptionChangeReceiptDTO | null>(null)
const confirmReason = ref('')
const manualApproval = ref(false)
const pending = ref(false)
const errorMessage = ref('')

const actionLabel = computed(() => ({ SWITCH: '切换套餐', RENEW: '续期', STOP_RENEWAL: '停止续期' })[action.value])
const previewExpired = computed(() => {
  if (!preview.value?.expiresAt) return false
  const expires = new Date(preview.value.expiresAt).valueOf()
  return Number.isFinite(expires) && expires <= Date.now()
})

watch(() => props.tenantId, reset)
watch(action, () => {
  preview.value = null
  receipt.value = null
  manualApproval.value = false
  confirmReason.value = ''
  errorMessage.value = ''
  if (action.value !== 'SWITCH' && props.subscription) {
    targetPlanCode.value = props.subscription.planCode
    targetPlanVersion.value = String(props.subscription.planVersion)
  }
})

function reset() {
  preview.value = null
  receipt.value = null
  errorMessage.value = ''
  manualApproval.value = false
  confirmReason.value = ''
  action.value = 'SWITCH'
  targetPlanCode.value = ''
  targetPlanVersion.value = ''
  effectiveAt.value = ''
  previewReason.value = ''
}

function toRfc3339(value: string) {
  if (!value) return ''
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? '' : parsed.toISOString()
}

function formatTime(value?: string) {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? value : parsed.toLocaleString('zh-CN', { hour12: false })
}

function planLabel(subscription?: TenantSubscriptionDTO) {
  if (!subscription) return '—'
  return `${subscription.planCode || '—'} v${subscription.planVersion || '—'}`
}

function limitLabel(limit?: { unlimited: boolean; value: string | number }) {
  if (!limit) return '—'
  return limit.unlimited ? 'unlimited' : String(limit.value ?? 0)
}

async function createPreview() {
  if (!props.tenantId || !props.subscription) return
  errorMessage.value = ''
  receipt.value = null

  const targetCode = action.value === 'SWITCH' ? targetPlanCode.value.trim() : props.subscription.planCode
  const targetVersion = action.value === 'SWITCH' ? targetPlanVersion.value.trim() : String(props.subscription.planVersion)
  if (!targetCode || !targetVersion) {
    errorMessage.value = '切换套餐必须输入真实目标 plan_code 与版本。'
    return
  }
  if (!previewReason.value.trim()) {
    errorMessage.value = '生成商业变更预览必须记录原因。'
    return
  }

  const normalizedEffectiveAt = toRfc3339(effectiveAt.value)
  if (effectiveAt.value && !normalizedEffectiveAt) {
    errorMessage.value = '生效时间格式无效。'
    return
  }

  pending.value = true
  try {
    const requestId = commercialRequestId('ce13-subscription-preview')
    preview.value = await previewSubscriptionChange(props.tenantId, {
      requestId,
      action: action.value,
      targetPlanCode: targetCode,
      targetPlanVersion: targetVersion,
      effectiveAt: normalizedEffectiveAt,
      reason: previewReason.value.trim(),
    })
    manualApproval.value = false
    confirmReason.value = ''
  } catch (error) {
    errorMessage.value = describeError(error, '套餐变更预览失败')
  } finally {
    pending.value = false
  }
}

async function confirmPreview() {
  if (!preview.value || !props.tenantId) return
  errorMessage.value = ''
  if (previewExpired.value) {
    errorMessage.value = '该预览已过期，不能确认；请重新生成预览。'
    preview.value = null
    return
  }
  if (!manualApproval.value) {
    errorMessage.value = '必须明确确认这是平台人工商业批准，而不是支付凭证。'
    return
  }
  if (!confirmReason.value.trim()) {
    errorMessage.value = '确认原因不能为空。'
    return
  }

  pending.value = true
  try {
    const requestId = commercialRequestId('ce13-subscription-confirm')
    receipt.value = await confirmSubscriptionChange(props.tenantId, preview.value.changeId, {
      requestId,
      previewHash: preview.value.previewHash,
      reason: confirmReason.value.trim(),
    })
  } catch (error) {
    if (error instanceof CommercialApiError && error.code === 'conflict') {
      preview.value = null
      emit('refresh')
      errorMessage.value = '订阅、权益或目录事实已变化，旧预览没有被强行确认；请重新生成预览。'
    } else {
      errorMessage.value = describeError(error, '套餐变更确认失败')
    }
  } finally {
    pending.value = false
  }
}

function describeError(error: unknown, fallback: string) {
  if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
    return `当前可信平台会话无权执行该操作：${error.message}`
  }
  return error instanceof Error ? error.message : fallback
}
</script>

<template>
  <section class="card change-card" data-testid="ce13-subscription-change">
    <header class="section-header">
      <div>
        <h2>套餐变更 Preview / Confirm</h2>
        <p>CE-09 平台人工变更链路；预览不改权益，确认也不等于支付凭证。</p>
      </div>
      <StatusBadge v-if="receipt" :text="receipt.status || 'RECEIPT'" :tone="receipt.status === 'APPLIED' ? 'success' : 'warning'" />
    </header>

    <div v-if="!subscription" class="empty-state">当前没有可用于变更的基础订阅。</div>

    <template v-else>
      <div class="form-grid">
        <label class="field" for="subscription-action"><span>操作</span><select id="subscription-action" v-model="action" class="input"><option value="SWITCH">切换套餐</option><option value="RENEW">续期</option><option value="STOP_RENEWAL">停止续期</option></select></label>
        <label class="field" for="change-target-code"><span>目标 plan_code</span><input id="change-target-code" v-model="targetPlanCode" class="input" :disabled="action !== 'SWITCH'" :placeholder="action === 'SWITCH' ? '例如 office-pro' : subscription.planCode" /></label>
        <label class="field" for="change-target-version"><span>目标版本</span><input id="change-target-version" v-model="targetPlanVersion" class="input" :disabled="action !== 'SWITCH'" :placeholder="action === 'SWITCH' ? '例如 2' : String(subscription.planVersion)" /></label>
        <label class="field" for="change-effective-at"><span>指定未来生效时间（可空）</span><input id="change-effective-at" v-model="effectiveAt" class="input" type="datetime-local" /></label>
        <label class="field wide" for="change-preview-reason"><span>预览原因</span><textarea id="change-preview-reason" v-model="previewReason" class="input" rows="2" :placeholder="`${actionLabel}的业务原因`" /></label>
      </div>
      <div class="form-actions">
        <span>当前：<strong>{{ planLabel(subscription) }}</strong> · revision {{ subscription.revision }} · pending {{ subscription.pendingChangeId || '无' }}</span>
        <button class="btn primary" type="button" :disabled="pending" @click="createPreview">{{ pending ? '处理中…' : '生成不可变预览' }}</button>
      </div>

      <p v-if="errorMessage" class="error" role="alert">{{ errorMessage }}</p>

      <section v-if="preview" class="preview-panel">
        <div class="preview-head">
          <div><strong>{{ preview.classification || preview.action }}</strong><span>{{ preview.mode || '—' }}</span></div>
          <div class="hash">change {{ preview.changeId }} · {{ preview.previewHash }}</div>
        </div>

        <div class="facts-grid">
          <div><span>源订阅</span><strong>{{ planLabel(preview.before) }}</strong></div>
          <div><span>目标</span><strong>{{ preview.target ? `${preview.target.planCode} v${preview.target.version}` : '—' }}</strong></div>
          <div><span>预计生效</span><strong>{{ formatTime(preview.effectiveAt) }}</strong></div>
          <div><span>预览到期</span><strong :class="{ expired: previewExpired }">{{ formatTime(preview.expiresAt) }}</strong></div>
          <div><span>source_version</span><strong>{{ preview.sourceVersion }}</strong></div>
          <div><span>entitlement_version</span><strong>{{ preview.entitlementVersion }}</strong></div>
          <div><span>目录修订</span><strong>{{ preview.catalogRevision }}</strong></div>
          <div><span>价格依据</span><strong>{{ preview.pricingBasis || '未配置价格引用；不代表免费' }}</strong></div>
        </div>

        <div v-if="preview.impacts?.length" class="impact-block"><h3>影响摘要</h3><ul><li v-for="impact in preview.impacts" :key="impact">{{ impact }}</li></ul></div>

        <div v-if="preview.dependencies?.length" class="impact-block"><h3>依赖变化</h3><ul><li v-for="item in preview.dependencies" :key="item.moduleCode"><code>{{ item.moduleCode }}</code> → {{ item.requiresModules?.join('、') || '无依赖' }}</li></ul></div>

        <div v-if="preview.quotaImpacts?.length" class="table-scroll">
          <table class="data-table">
            <thead><tr><th>额度</th><th>切换前</th><th>切换后</th><th>使用量</th><th>策略</th></tr></thead>
            <tbody><tr v-for="item in preview.quotaImpacts" :key="`${item.moduleCode}:${item.key}`"><td><strong>{{ item.moduleCode }}</strong><small>{{ item.key }}</small></td><td>{{ limitLabel(item.beforeLimit) }}</td><td>{{ limitLabel(item.afterLimit) }}</td><td>{{ item.usageKnown ? item.used : '未知' }}<small v-if="item.overLimit" class="danger-text">超额</small></td><td>{{ item.policy || '—' }}<small>{{ item.evidence || '—' }}</small></td></tr></tbody>
          </table>
        </div>

        <div v-if="preview.provisioningRequirements?.length" class="impact-block"><h3>开通要求</h3><ul><li v-for="item in preview.provisioningRequirements" :key="item.code">{{ item.code }} · {{ item.adapter }}@{{ item.version }} · max {{ item.maxAttempts }}</li></ul></div>

        <div class="authority-note" :class="{ warning: preview.quotaValidationRequired }">
          <strong>{{ preview.quotaValidationRequired ? '确认前仍需额度再校验' : '服务端已形成可确认预览' }}</strong>
          <span>投影权益不是当前授权快照；确认时服务端仍会重新校验订阅、来源、目录和目标资格。</span>
        </div>

        <div class="confirm-box">
          <label class="check"><input v-model="manualApproval" type="checkbox" />我确认这是 <strong>PLATFORM_MANUAL_APPROVAL</strong>，不把它当作付款成功证明。</label>
          <label class="field" for="change-confirm-reason"><span>确认原因</span><textarea id="change-confirm-reason" v-model="confirmReason" class="input" rows="2" placeholder="说明为什么批准本次商业变更" /></label>
          <button class="btn primary" type="button" :disabled="pending || previewExpired" @click="confirmPreview">确认此 preview_hash</button>
        </div>
      </section>

      <section v-if="receipt" class="receipt-panel">
        <header><div><h3>不可变变更回执</h3><p>{{ receipt.changeId }} · {{ receipt.previewHash }}</p></div><StatusBadge :text="receipt.status || 'UNKNOWN'" :tone="receipt.status === 'APPLIED' ? 'success' : 'warning'" /></header>
        <div class="facts-grid"><div><span>模式</span><strong>{{ receipt.mode || '—' }}</strong></div><div><span>确认时间</span><strong>{{ formatTime(receipt.confirmedAt) }}</strong></div><div><span>实际/预约生效</span><strong>{{ formatTime(receipt.effectiveAt) }}</strong></div><div><span>变更后套餐</span><strong>{{ planLabel(receipt.after) }}</strong></div><div><span>source_version</span><strong>{{ receipt.beforeSourceVersion }} → {{ receipt.afterSourceVersion }}</strong></div><div><span>entitlement_version</span><strong>{{ receipt.beforeEntitlementVersion }} → {{ receipt.afterEntitlementVersion }}</strong></div><div><span>pricing authority</span><strong>{{ receipt.pricingAuthority || '—' }}</strong></div><div><span>provisioning task</span><strong>{{ receipt.provisioningTaskId || '无' }}</strong></div></div>
        <p v-if="receipt.status === 'SCHEDULED'" class="scheduled-note">SCHEDULED 只表示预约已保存；当前套餐和权益尚未切换，后续执行器必须重新校验后才能应用。</p>
      </section>
    </template>
  </section>
</template>

<style scoped>
.change-card { overflow: hidden; }
.section-header, .receipt-panel header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.section-header h2, .receipt-panel h3 { margin: 0; font-size: 16px; }
.section-header p, .receipt-panel p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.empty-state { padding: 24px 20px; color: var(--color-text-muted); }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 18px 20px 10px; }
.field { display: grid; gap: 6px; font-size: 12px; color: var(--color-text-secondary); }
.field.wide { grid-column: 1 / -1; }
.input { width: 100%; min-height: 36px; border: 1px solid var(--color-border); border-radius: 7px; padding: 7px 10px; background: var(--color-surface); color: var(--color-text-primary); font: inherit; }
.input:disabled { opacity: .65; }
.form-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 20px 18px; font-size: 12px; color: var(--color-text-secondary); }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: white; }
.error { margin: 0 20px 18px; padding: 10px 12px; border-radius: 8px; color: var(--color-danger, #b42318); background: rgba(180, 35, 24, .08); }
.preview-panel, .receipt-panel { border-top: 1px solid var(--color-border); }
.preview-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 20px; background: var(--color-surface-subtle); }
.preview-head > div:first-child { display: flex; align-items: center; gap: 8px; }
.preview-head span { font-size: 12px; color: var(--color-text-muted); }
.hash { max-width: 55%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; color: var(--color-text-muted); }
.facts-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1px; background: var(--color-border); }
.facts-grid > div { min-width: 0; padding: 13px 14px; background: var(--color-surface); }
.facts-grid span, td small { display: block; color: var(--color-text-muted); font-size: 11px; }
.facts-grid strong { display: block; margin-top: 4px; font-size: 13px; word-break: break-word; }
.expired, .danger-text { color: var(--color-danger, #b42318) !important; }
.impact-block { padding: 16px 20px; border-top: 1px solid var(--color-border); }
.impact-block h3 { margin: 0 0 8px; font-size: 13px; }
.impact-block ul { margin: 0; padding-left: 20px; color: var(--color-text-secondary); font-size: 12px; }
.impact-block li + li { margin-top: 5px; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.authority-note { display: grid; gap: 4px; margin: 16px 20px; padding: 12px; border: 1px solid var(--color-success, #4b9a68); border-radius: 8px; font-size: 12px; }
.authority-note.warning { border-color: var(--color-warning, #d9a441); }
.authority-note span { color: var(--color-text-secondary); }
.confirm-box { display: grid; gap: 12px; padding: 0 20px 20px; }
.check { display: flex; align-items: flex-start; gap: 8px; font-size: 12px; color: var(--color-text-secondary); }
.check input { margin-top: 2px; }
.receipt-panel { margin-top: 0; }
.scheduled-note { margin: 14px 20px 18px !important; padding: 12px; border: 1px solid var(--color-warning, #d9a441); border-radius: 8px; color: var(--color-text-secondary) !important; }
@media (max-width: 900px) { .facts-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 700px) { .form-grid, .facts-grid { grid-template-columns: 1fr; } .form-actions, .preview-head, .section-header, .receipt-panel header { align-items: flex-start; flex-direction: column; } .hash { max-width: 100%; } }
</style>
