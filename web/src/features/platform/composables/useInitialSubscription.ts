import { computed, onScopeDispose, ref, watch } from 'vue'
import { initialSubscriptionReadbackMatches } from '@/services/commercial/initialSubscriptionOutcome'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import { backendStateTone } from '@/i18n/backend-terms'
import {
  CommercialApiError,
  checkPlanEligibility,
  commercialRequestId,
  confirmSubscriptionChange,
  explainTenantEntitlements,
  getProvisioningTask,
  getSubscriptionChangeReceipt,
  getTenantSubscription,
  listPlans,
  listPlanVersions,
  previewSubscriptionChange,
  retryProvisioningTask,
  type PlanCatalogEntryDTO,
  type PlanVersionDTO,
  type ProvisioningTaskDTO,
  type SubscriptionChangePreviewDTO,
  type SubscriptionChangeReceiptDTO,
  type TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'

export type InitialSubscriptionTargetCandidate = {
  version: PlanVersionDTO
  eligible: boolean
  reason: string
}

type VerificationState = 'idle' | 'pending' | 'verified' | 'failed'

export function useInitialSubscription(tenantId: () => string, onRefresh: () => void) {
  const plans = ref<PlanCatalogEntryDTO[]>([])
  const salesScope = ref('')
  const planCode = ref('')
  const candidates = ref<InitialSubscriptionTargetCandidate[]>([])
  const selectedVersionKey = ref('')
  const previewReason = ref('')
  const confirmReason = ref('')
  const approved = ref(false)
  const preview = ref<SubscriptionChangePreviewDTO | null>(null)
  const receipt = ref<SubscriptionChangeReceiptDTO | null>(null)
  const task = ref<ProvisioningTaskDTO | null>(null)
  const finalSubscription = ref<TenantSubscriptionDTO | null>(null)
  const verificationState = ref<VerificationState>('idle')
  const loadingPlans = ref(false)
  const loadingTargets = ref(false)
  const pending = ref(false)
  const errorMessage = ref('')
  const statusMessage = ref('')
  const confirmationSubmitted = ref(false)
  let contextEpoch = 0
  let readbackEpoch = 0
  let disposed = false

  const selectedCandidate = computed(() =>
    candidates.value.find((item) => versionKey(item.version) === selectedVersionKey.value) ?? null,
  )
  const selectedVersion = computed(() => selectedCandidate.value?.version ?? null)
  const eligibleCandidates = computed(() => candidates.value.filter((item) => item.eligible))
  const provisioningStatus = computed(() => {
    switch (task.value?.state) {
    case 'QUEUED': return '等待处理'
    case 'RUNNING': return '处理中'
    case 'RETRY_WAIT': return '等待重试'
    case 'READY': return '准备完成，等待最终生效'
    case 'APPLIED': return '已完成'
    case 'FAILED': return '处理失败'
    case 'RECONCILIATION_REQUIRED': return '结果待确认'
    default: return task.value ? '状态待确认' : '尚未产生准备任务'
    }
  })
  const receiptTone = computed(() => receipt.value
    ? backendStateTone('receiptStatus', receipt.value.status)
    : 'neutral',
  )

  function versionKey(version: PlanVersionDTO) {
    return `${version.planCode}:${version.version}`
  }

  function formatValidity(version?: PlanVersionDTO | null) {
    const terms = version?.terms
    if (!terms) return '—'
    if (terms.validityMode === 'unlimited') return '长期有效'
    if (terms.validityMode === 'fixed_days') return `${terms.validityDays} 天`
    return '按套餐条款'
  }

  function limitLabel(value: { unlimited: boolean; value: string | number }) {
    return value.unlimited ? '不限' : String(value.value)
  }

  function clearFlow(keepTarget = false) {
    contextEpoch++
    readbackEpoch++
    pending.value = false
    loadingPlans.value = false
    loadingTargets.value = false
    confirmationSubmitted.value = false
    preview.value = null
    receipt.value = null
    task.value = null
    finalSubscription.value = null
    verificationState.value = 'idle'
    previewReason.value = ''
    confirmReason.value = ''
    approved.value = false
    errorMessage.value = ''
    statusMessage.value = ''
    if (!keepTarget) {
      candidates.value = []
      selectedVersionKey.value = ''
    }
  }

  function reset() {
    salesScope.value = ''
    planCode.value = ''
    plans.value = []
    clearFlow(false)
  }

  watch(tenantId, reset, { flush: 'sync' })
  watch(salesScope, () => clearFlow(false), { flush: 'sync' })
  watch(planCode, () => clearFlow(false), { flush: 'sync' })
  watch(selectedVersionKey, () => clearFlow(true), { flush: 'sync' })
  const unsubscribe = subscribeSessionContextChange(reset)
  onScopeDispose(() => {
    disposed = true
    readbackEpoch++
    unsubscribe()
  })

  function captureContext() {
    const generation = contextEpoch
    const tenant = tenantId()
    return { tenant, current: () => !disposed && generation === contextEpoch && tenantId() === tenant }
  }

  async function loadPlanCatalog() {
    if (loadingPlans.value || pending.value || confirmationSubmitted.value || disposed) return
    const context = captureContext()
    loadingPlans.value = true
    errorMessage.value = ''
    try {
      const found: PlanCatalogEntryDTO[] = []
      let after = ''
      for (let page = 0; page < 20; page++) {
        const result = await listPlans({ afterPlanCode: after || undefined, pageSize: 100 })
        if (!context.current()) return
        found.push(...result.plans)
        const next = result.nextAfterPlanCode.trim()
        if (!next || next === after) break
        after = next
      }
      const unique = new Map(found.map((item) => [item.planCode, item]))
      plans.value = [...unique.values()].sort((a, b) => a.planCode.localeCompare(b.planCode))
      if (!plans.value.length) statusMessage.value = '当前没有可发现的套餐目录。'
    } catch (error) {
      if (context.current()) errorMessage.value = describeError(error, '套餐目录读取失败。')
    } finally {
      if (context.current()) loadingPlans.value = false
    }
  }

  async function loadPublishedVersions() {
    if (loadingTargets.value || pending.value || confirmationSubmitted.value || disposed) return
    const scope = salesScope.value.trim()
    const code = planCode.value.trim()
    clearFlow(false)
    if (!scope || scope === '*') {
      errorMessage.value = '请填写具体适用范围，不能使用通配范围。'
      return
    }
    if (!code) {
      errorMessage.value = '请选择套餐。'
      return
    }

    const context = captureContext()
    loadingTargets.value = true
    try {
      const versions: PlanVersionDTO[] = []
      let after: string | number | undefined
      for (let page = 0; page < 20; page++) {
        const result = await listPlanVersions(code, { afterVersion: after, pageSize: 50 })
        if (!context.current()) return
        versions.push(...result.versions.filter((item) => item.state === 'PUBLISHED'))
        const next = result.nextAfterVersion
        if (next === '' || next === undefined || String(next) === String(after ?? '')) break
        after = next
      }

      const checked = await Promise.all(versions.map(async (version): Promise<InitialSubscriptionTargetCandidate> => {
        try {
          const result = await checkPlanEligibility(version.planCode, version.version, scope)
          return { version, eligible: Boolean(result.eligible), reason: result.reason || '' }
        } catch (error) {
          if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) throw error
          return { version, eligible: false, reason: '当前版本资格暂无法确认' }
        }
      }))

      if (!context.current()) return
      loadingTargets.value = false
      candidates.value = checked.sort((left, right) => Number(right.version.version) - Number(left.version.version))
      const firstEligible = checked.find((item) => item.eligible)
      selectedVersionKey.value = firstEligible ? versionKey(firstEligible.version) : ''
      statusMessage.value = checked.length
        ? (eligibleCandidates.value.length ? '请选择一个精确已发布版本继续。' : '当前套餐没有符合该适用范围的可开通版本。')
        : '当前套餐没有已发布版本。'
    } catch (error) {
      if (context.current()) errorMessage.value = describeError(error, '套餐版本资格检查失败。')
    } finally {
      if (context.current()) loadingTargets.value = false
    }
  }

  async function createPreview() {
    if (pending.value || confirmationSubmitted.value || disposed) return
    const context = captureContext()
    const target = selectedVersion.value
    if (!target || !selectedCandidate.value?.eligible) {
      errorMessage.value = '请选择当前资格检查确认可适用的已发布版本。'
      return
    }
    if (!previewReason.value.trim()) {
      errorMessage.value = '请填写首次开通原因。'
      return
    }

    pending.value = true
    errorMessage.value = ''
    statusMessage.value = ''
    receipt.value = null
    task.value = null
    verificationState.value = 'idle'
    try {
      const result = await previewSubscriptionChange(context.tenant, {
        requestId: commercialRequestId('platform-initial-preview'),
        action: 'INITIAL',
        salesScope: salesScope.value.trim(),
        targetPlanCode: target.planCode,
        targetPlanVersion: target.version,
        effectiveAt: '',
        reason: previewReason.value.trim(),
      })
      if (!context.current()) return
      preview.value = result
      approved.value = false
      confirmReason.value = ''
      statusMessage.value = '首次开通方案已生成；确认前系统仍会重新核对套餐、目录和当前租户事实。'
    } catch (error) {
      if (context.current()) errorMessage.value = describeError(error, '首次开通方案生成失败。')
    } finally {
      if (context.current()) pending.value = false
    }
  }

  async function confirmPreview() {
    if (pending.value || confirmationSubmitted.value || disposed) return
    const context = captureContext()
    if (!preview.value || !selectedVersion.value) return
    if (!approved.value) {
      errorMessage.value = '请先确认已核对首次开通影响。'
      return
    }
    if (!confirmReason.value.trim()) {
      errorMessage.value = '确认原因不能为空。'
      return
    }

    pending.value = true
    errorMessage.value = ''
    statusMessage.value = ''
    confirmationSubmitted.value = true
    try {
      const confirmed = await confirmSubscriptionChange(context.tenant, preview.value.changeId, {
        requestId: commercialRequestId('platform-initial-confirm'),
        previewHash: preview.value.previewHash,
        reason: confirmReason.value.trim(),
      })
      if (!context.current()) return
      receipt.value = confirmed
      if (receipt.value.status === 'APPLIED') {
        await verifyReadback()
      } else if (receipt.value.status === 'PROVISIONING' && receipt.value.provisioningTaskId) {
        verificationState.value = 'pending'
        const prepared = await getProvisioningTask(context.tenant, receipt.value.provisioningTaskId)
        if (!context.current()) return
        task.value = prepared
        statusMessage.value = '首次开通已进入准备流程；套餐权益尚未正式生效。'
      } else {
        verificationState.value = 'pending'
        statusMessage.value = '首次开通结果仍在确认中，请重新读取结果。'
      }
    } catch (error) {
      if (!context.current()) return
      if (error instanceof CommercialApiError && error.code === 'conflict') {
        await recoverConcurrentActivation(context)
      } else {
        if (!receipt.value && error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
          confirmationSubmitted.value = false
        }
        errorMessage.value = describeError(error, '首次开通确认结果未知。请先读取原变更结果，不要重复提交。')
      }
    } finally {
      if (context.current()) pending.value = false
    }
  }

  async function verifyReadback() {
    const confirmed = receipt.value
    const selected = selectedVersion.value
    const proposed = preview.value
    if (!confirmed || !selected || !proposed) return
    const active = captureContext()
    const context = { tenantId: active.tenant, salesScope: salesScope.value.trim(), target: selected, preview: proposed, receipt: confirmed }
    const generation = ++readbackEpoch
    const current = () => active.current() && generation === readbackEpoch
      && tenantId() === context.tenantId && receipt.value === confirmed
      && preview.value === proposed && selectedVersion.value === selected
    verificationState.value = 'pending'
    finalSubscription.value = null
    statusMessage.value = ''
    try {
      const [subscription, entitlements] = await Promise.all([
        getTenantSubscription(context.tenantId),
        explainTenantEntitlements(context.tenantId, []),
      ])
      if (!current()) return
      if (!initialSubscriptionReadbackMatches({ ...context, subscription, entitlements })) {
        verificationState.value = 'failed'
        errorMessage.value = '订阅回执已存在，但最终权益尚未完成一致性确认。请重新读取结果，不要重复首次开通。'
        return
      }
      finalSubscription.value = subscription
      verificationState.value = 'verified'
      errorMessage.value = ''
      statusMessage.value = '首次开通已完成，并已从最终权益结果确认目标套餐能力。'
      onRefresh()
    } catch {
      if (!current()) return
      verificationState.value = 'failed'
      errorMessage.value = '首次开通回执已保留，但最终权益读取失败。请重新读取结果，不要重复提交。'
    }
  }

  async function readResult(context: ReturnType<typeof captureContext>) {
    const changeId = preview.value?.changeId
    if (!changeId || !context.current()) return
    const latest = await getSubscriptionChangeReceipt(context.tenant, changeId)
    if (!context.current()) return
    receipt.value = latest
    if (latest.status === 'APPLIED') {
      await verifyReadback()
      return
    }
    verificationState.value = 'pending'
    if (latest.provisioningTaskId) {
      const prepared = await getProvisioningTask(context.tenant, latest.provisioningTaskId)
      if (!context.current()) return
      task.value = prepared
      if (prepared.state === 'APPLIED') {
        const applied = await getSubscriptionChangeReceipt(context.tenant, changeId)
        if (!context.current()) return
        receipt.value = applied
        if (applied.status === 'APPLIED') await verifyReadback()
      }
      return
    }
    statusMessage.value = '当前结果仍未形成最终权益事实。'
  }

  async function refreshResult() {
    if (!preview.value || pending.value || disposed) return
    const context = captureContext()
    pending.value = true
    errorMessage.value = ''
    statusMessage.value = ''
    verificationState.value = 'pending'
    try {
      await readResult(context)
    } catch (error) {
      if (context.current()) errorMessage.value = describeError(error, '结果读取失败。')
    } finally {
      if (context.current()) pending.value = false
    }
  }

  async function retryTask() {
    if (pending.value || disposed || !task.value?.retryAllowed || !receipt.value?.provisioningTaskId) return
    const context = captureContext()
    pending.value = true
    errorMessage.value = ''
    try {
      const prepared = await retryProvisioningTask(context.tenant, receipt.value.provisioningTaskId, {
        requestId: commercialRequestId('platform-initial-retry'),
        expectedRevision: task.value.revision,
        reason: '继续首次开通准备任务',
      })
      if (!context.current()) return
      task.value = prepared
      statusMessage.value = '已恢复原准备任务；不会创建第二份首次订阅。'
    } catch (error) {
      if (!context.current()) return
      if (error instanceof CommercialApiError && error.code === 'conflict') {
        try { await readResult(context) } catch (readError) {
          if (context.current()) errorMessage.value = describeError(readError, '结果读取失败。')
        }
      } else {
        errorMessage.value = describeError(error, '准备任务恢复失败。')
      }
    } finally {
      if (context.current()) pending.value = false
    }
  }

  async function recoverConcurrentActivation(context: ReturnType<typeof captureContext>) {
    try {
      const existing = await getTenantSubscription(context.tenant)
      if (!context.current()) return
      if (existing?.subscriptionId && existing.tenantId === context.tenant) {
        statusMessage.value = '该租户已被其他管理员完成或接管首次开通，正在切换到当前真实订阅。'
        onRefresh()
        return
      }
    } catch {
      // Preserve the conflict if the authoritative result cannot be read.
    }
    if (!context.current()) return
    errorMessage.value = '首次开通事实已发生变化。请重新读取租户状态后再继续，不会覆盖其他管理员的操作。'
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof CommercialApiError && error.code === 'unauthenticated') {
      return '当前平台会话已失效，请重新登录后读取最新状态。'
    }
    if (error instanceof CommercialApiError && error.code === 'forbidden') {
      return '当前平台账号无权执行首次开通，请使用具备套餐与订阅管理权限的账号。'
    }
    return error instanceof Error ? error.message : fallback
  }

  return {
    plans,
    salesScope,
    planCode,
    candidates,
    selectedVersionKey,
    previewReason,
    confirmReason,
    approved,
    preview,
    receipt,
    task,
    finalSubscription,
    verificationState,
    loadingPlans,
    loadingTargets,
    pending,
    confirmationSubmitted,
    errorMessage,
    statusMessage,
    selectedVersion,
    eligibleCandidates,
    provisioningStatus,
    receiptTone,
    versionKey,
    formatValidity,
    limitLabel,
    loadPlanCatalog,
    loadPublishedVersions,
    createPreview,
    confirmPreview,
    refreshResult,
    retryTask,
  }
}
