import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { backendErrorFallback } from '@/i18n/backend-terms'
import { CommercialApiError, type PlanVersionDTO, type SubscriptionChangePreviewDTO, type SubscriptionChangeReceiptDTO, type TenantSubscriptionDTO } from '@/services/commercial/platformCommercial'
import { sessionContext, type TrustedSession } from '@/services/runtime/api'
import {
  confirmMySubscriptionChange, createTenantChangeRequestId, getMySubscriptionChangeReceipt,
  listMySubscriptionChangeTargets, needsExternalCommercialApproval,
  previewMySubscriptionChange, tenantChangeRuntimeError, type TenantChangeAction,
} from '@/services/enterprise/planChangeRuntime'

export function usePlanChangeFlow(
  props: { session: TrustedSession; subscription: TenantSubscriptionDTO; blocked?: boolean },
  changed: () => void,
) {
  const opened = ref(false)
  const action = ref<TenantChangeAction>('SWITCH')
  const targets = ref<PlanVersionDTO[]>([])
  const targetsLoading = ref(false)
  const selectedTarget = ref<PlanVersionDTO | null>(null)
  const effectiveAt = ref('')
  const preview = ref<SubscriptionChangePreviewDTO | null>(null)
  const receipt = ref<SubscriptionChangeReceiptDTO | null>(null)
  const working = ref(false)
  const errorMessage = ref('')
  const receiptRefreshing = ref(false)
  const confirmationOpen = ref(false)
  const confirmationUnknown = ref(false)
  const stage = computed(() => receipt.value ? 'receipt' : preview.value ? 'preview' : 'select')
  const approvalRequired = computed(() => needsExternalCommercialApproval(preview.value))
  const canPreview = computed(() => !props.blocked && !working.value && !confirmationUnknown.value && (action.value !== 'SWITCH' || Boolean(selectedTarget.value)))
  const context = computed(() => sessionContext(props.session))
  let generation = 0
  let receiptTimer: ReturnType<typeof setInterval> | undefined
  let restoredPendingKey = ''
  let confirmRequestId = ''

  const current = (ticket: number) => ticket === generation
  const pending = (value: SubscriptionChangeReceiptDTO | null) => value?.status === 'SCHEDULED' || value?.status === 'PROVISIONING'
  const explain = (cause: unknown) => cause instanceof CommercialApiError ? tenantChangeRuntimeError(cause) : backendErrorFallback('planChange')
  function stopReceiptSync() {
    if (receiptTimer) clearInterval(receiptTimer)
    receiptTimer = undefined
  }
  function startReceiptSync() {
    stopReceiptSync()
    if (pending(receipt.value)) receiptTimer = setInterval(() => { void refreshReceipt() }, 15_000)
  }
  function acceptReceipt(value: SubscriptionChangeReceiptDTO, id: string) {
    if (value.changeId !== id || value.tenantId !== props.session.active_tenant_id) throw new Error('Receipt scope mismatch')
    receipt.value = value
    confirmationUnknown.value = false
    restoredPendingKey = `${context.value}:${id}`
    if (pending(value)) startReceiptSync()
    else { stopReceiptSync(); changed() }
  }
  function resetLifecycle() {
    if (working.value || confirmationUnknown.value) return
    stopReceiptSync()
    confirmationOpen.value = false
    preview.value = null
    receipt.value = null
    errorMessage.value = ''
    confirmRequestId = ''
  }
  watch(action, () => {
    selectedTarget.value = null
    resetLifecycle()
    if (action.value === 'STOP_RENEWAL') effectiveAt.value = ''
  })
  watch(context, () => {
    generation += 1
    stopReceiptSync()
    opened.value = false
    targets.value = []
    targetsLoading.value = false
    selectedTarget.value = null
    effectiveAt.value = ''
    preview.value = null
    receipt.value = null
    errorMessage.value = ''
    working.value = false
    receiptRefreshing.value = false
    confirmationUnknown.value = false
    confirmationOpen.value = false
    restoredPendingKey = ''
    confirmRequestId = ''
  }, { flush: 'sync' })
  watch(() => props.blocked, (blocked) => {
    if (blocked) confirmationOpen.value = false
  }, { flush: 'sync' })

  async function openLifecycle() {
    opened.value = true
    if (props.blocked || targetsLoading.value || targets.value.length > 0) return
    const ticket = generation
    targetsLoading.value = true
    errorMessage.value = ''
    try {
      const result = await listMySubscriptionChangeTargets(props.session)
      if (current(ticket)) targets.value = Array.isArray(result.targets) ? result.targets : []
    } catch (cause) {
      if (current(ticket)) errorMessage.value = explain(cause)
    } finally {
      if (current(ticket)) targetsLoading.value = false
    }
  }

  async function restorePendingChange() {
    const id = String(props.subscription.pendingChangeId ?? '').trim()
    const key = `${context.value}:${id}`
    if (!id || props.blocked || !props.session.active_tenant_id || props.session.active_tenant_id !== props.subscription.tenantId || key === restoredPendingKey || receiptRefreshing.value) return
    const ticket = generation
    restoredPendingKey = key
    opened.value = true
    receiptRefreshing.value = true
    errorMessage.value = ''
    try {
      const recovered = await getMySubscriptionChangeReceipt(props.session, id)
      if (!current(ticket)) return
      confirmationOpen.value = false
      preview.value = null
      acceptReceipt(recovered, id)
    } catch (cause) {
      if (current(ticket)) { restoredPendingKey = ''; errorMessage.value = explain(cause) }
    } finally {
      if (current(ticket)) receiptRefreshing.value = false
    }
  }

  async function refreshReceipt() {
    const id = receipt.value?.changeId || (confirmationUnknown.value ? preview.value?.changeId : '')
    if (!id || receiptRefreshing.value) return
    const ticket = generation
    receiptRefreshing.value = true
    errorMessage.value = ''
    try {
      const value = await getMySubscriptionChangeReceipt(props.session, id)
      if (current(ticket)) acceptReceipt(value, id)
    } catch (cause) {
      if (current(ticket)) { stopReceiptSync(); errorMessage.value = explain(cause) }
    } finally {
      if (current(ticket)) receiptRefreshing.value = false
    }
  }

  async function createPreview() {
    if (!canPreview.value) return
    const ticket = generation
    working.value = true
    errorMessage.value = ''
    receipt.value = null
    try {
      const date = effectiveAt.value ? new Date(effectiveAt.value) : null
      const value = await previewMySubscriptionChange(props.session, {
        action: action.value,
        targetPlanCode: selectedTarget.value?.planCode,
        targetPlanVersion: selectedTarget.value?.version,
        effectiveAt: action.value !== 'STOP_RENEWAL' && date && !Number.isNaN(date.getTime()) ? date.toISOString() : '',
      })
      if (!current(ticket) || props.blocked) return
      if (value.tenantId !== props.session.active_tenant_id || value.action !== action.value) throw new Error('Preview scope mismatch')
      preview.value = value
      confirmRequestId = createTenantChangeRequestId('tenant-plan-confirm')
    } catch (cause) {
      if (current(ticket)) { preview.value = null; errorMessage.value = explain(cause) }
    } finally {
      if (current(ticket)) working.value = false
    }
  }

  async function confirmPreview() {
    const value = preview.value
    if (!value || props.blocked || working.value || confirmationUnknown.value || approvalRequired.value || value.quotaValidationRequired) return
    const ticket = generation
    working.value = true
    errorMessage.value = ''
    confirmationOpen.value = false
    try {
      const result = await confirmMySubscriptionChange(props.session, value, confirmRequestId)
      if (current(ticket)) acceptReceipt(result, value.changeId)
    } catch (cause) {
      if (!current(ticket)) return
      errorMessage.value = explain(cause)
      // An unconfirmed transport outcome must be queried using the original ID;
      // the read-recovery action never creates another confirmation request.
      confirmationUnknown.value = !(cause instanceof CommercialApiError && [400, 401, 403, 409, 422].includes(cause.status))
    } finally {
      if (current(ticket)) working.value = false
    }
  }

  watch(() => [context.value, props.subscription.pendingChangeId, props.blocked].join(':'), () => { void restorePendingChange() }, { immediate: true })
  onBeforeUnmount(() => { generation += 1; stopReceiptSync() })
  return {
    opened, action, targets, targetsLoading, selectedTarget, effectiveAt,
    preview, receipt, working, errorMessage, receiptRefreshing, confirmationOpen,
    confirmationUnknown, stage, approvalRequired, canPreview,
    openLifecycle, resetLifecycle, refreshReceipt, createPreview, confirmPreview,
  }
}
