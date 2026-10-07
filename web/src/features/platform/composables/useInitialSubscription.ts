import { computed, onMounted, ref, watch } from 'vue'
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
  type EntitlementView,
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

type ReadbackState = 'idle' | 'pending' | 'verified' | 'failed'

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
  const readbackState = ref<ReadbackState>('idle')
  const loadingPlans = ref(false)
  const loadingTargets = ref(false)
  const pending = ref(false)
  const errorMessage = ref('')
  const statusMessage = ref('')

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
    preview.value = null
    receipt.value = null
    task.value = null
    finalSubscription.value = null
    readbackState.value = 'idle'
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
    void loadPlanCatalog()
  }

  watch(tenantId, reset)
  watch(salesScope, () => clearFlow(false))
  watch(planCode, () => clearFlow(false))
  watch(selectedVersionKey, () => clearFlow(true))

  async function loadPlanCatalog() {
    loadingPlans.value = true
    errorMessage.value = ''
    try {
      const found: PlanCatalogEntryDTO[] = []
      let after = ''
      for (let page = 0; page < 20; page++) {
        const result = await listPlans({ afterPlanCode: after || undefined, pageSize: 100 })
        found.push(...result.plans)
        const next = result.nextAfterPlanCode.trim()
        if (!next || next === after) break
        after = next
      }
      const unique = new Map(found.map((item) => [item.planCode, item]))
      plans.value = [...unique.values()].sort((a, b) => a.planCode.localeCompare(b.planCode))
      if (!plans.value.length) statusMessage.value = '当前没有可发现的套餐目录。'
    } catch (error) {
      errorMessage.value = describeError(error, '套餐目录读取失败。')
    } finally {
      loadingPlans.value = false
    }
  }

  async function loadPublishedVersions() {
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

    loadingTargets.value = true
    try {
      const versions: PlanVersionDTO[] = []
      let after: string | number | undefined
      for (let page = 0; page < 20; page++) {
        const result = await listPlanVersions(code, { afterVersion: after, pageSize: 50 })
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

      candidates.value = checked.sort((left, right) => Number(right.version.version) - Number(left.version.version))
      const firstEligible = checked.find((item) => item.eligible)
      selectedVersionKey.value = firstEligible ? versionKey(firstEligible.version) : ''
      statusMessage.value = checked.length
        ? (eligibleCandidates.value.length ? '请选择一个精确已发布版本继续。' : '当前套餐没有符合该适用范围的可开通版本。')
        : '当前套餐没有已发布版本。'
    } catch (error) {
      errorMessage.value = describeError(error, '套餐版本资格检查失败。')
    } finally {
      loadingTargets.value = false
    }
  }

  async function createPreview() {
    const target = selectedVersion.value
    if (!target || !selectedCandidate.value?.eligible) {
      errorMessage.value = '请选择服务端确认可适用的已发布版本。'
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
    readbackState.value = 'idle'
    try {
      preview.value = await previewSubscriptionChange(tenantId(), {
        requestId: commercialRequestId('platform-initial-preview'),
        action: 'INITIAL',
        salesScope: salesScope.value.trim(),
        targetPlanCode: target.planCode,
        targetPlanVersion: target.version,
        effectiveAt: '',
        reason: previewReason.value.trim(),
      })
      approved.value = false
      confirmReason.value = ''
      statusMessage.value = '首次开通方案已生成；确认前系统仍会重新核对套餐、目录和当前租户事实。'
    } catch (error) {
      errorMessage.value = describeError(error, '首次开通方案生成失败。')
    } finally {
      pending.value = false
    }
  }

  async function confirmPreview() {
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
    try {
      receipt.value = await confirmSubscriptionChange(tenantId(), preview.value.changeId, {
        requestId: commercialRequestId('platform-initial-confirm'),
        previewHash: preview.value.previewHash,
        reason: confirmReason.value.trim(),
      })
      if (receipt.value.status === 'APPLIED') {
        await verifyReadback()
      } else if (receipt.value.status === 'PROVISIONING' && receipt.value.provisioningTaskId) {
        readbackState.value = 'pending'
        task.value = await getProvisioningTask(tenantId(), receipt.value.provisioningTaskId)
        statusMessage.value = '首次开通已进入准备流程；套餐权益尚未正式生效。'
      } else {
        readbackState.value = 'pending'
        statusMessage.value = '首次开通结果仍在确认中，请重新读取结果。'
      }
    } catch (error) {
      if (error instanceof CommercialApiError && error.code === 'conflict') {
        await recoverConcurrentActivation()
      } else {
        errorMessage.value = describeError(error, '首次开通确认失败。')
      }
    } finally {
      pending.value = false
    }
  }

  function versionAtLeast(current: string | number, expected: string | number) {
    const a = Number(current)
    const b = Number(expected)
    if (Number.isFinite(a) && Number.isFinite(b)) return a >= b
    return String(current) === String(expected)
  }

  function expectedDecisionCoverage(view: EntitlementView, target: PlanVersionDTO) {
    const decisions = view.decisions ?? []
    for (const module of target.terms?.modules ?? []) {
      const moduleDecisions = decisions.filter((item) => item.moduleCode === module.moduleCode)
      if (!moduleDecisions.length) return false
      for (const capability of module.capabilityCodes ?? []) {
        if (!moduleDecisions.some((item) => item.kind === 'capability' && item.key === capability)) return false
      }
      for (const quota of module.quotas ?? []) {
        if (!moduleDecisions.some((item) => item.kind === 'quota' && item.key === quota.key)) return false
      }
    }
    return true
  }

  async function verifyReadback() {
    if (!receipt.value || !selectedVersion.value) return
    readbackState.value = 'pending'
    try {
      const [subscription, entitlements] = await Promise.all([
        getTenantSubscription(tenantId()),
        explainTenantEntitlements(tenantId(), []),
      ])
      finalSubscription.value = subscription

      const targetMatches = subscription.planCode === selectedVersion.value.planCode
        && String(subscription.planVersion) === String(selectedVersion.value.version)
        && subscription.state !== 'PROVISIONING'
      const versionsMatch = versionAtLeast(entitlements.sourceVersion, receipt.value.afterSourceVersion)
        && versionAtLeast(entitlements.entitlementVersion, receipt.value.afterEntitlementVersion)
      const decisionsMatch = expectedDecisionCoverage(entitlements, selectedVersion.value)

      if (!targetMatches || !versionsMatch || !decisionsMatch) {
        readbackState.value = 'failed'
        statusMessage.value = ''
        errorMessage.value = '订阅回执已存在，但最终权益尚未完成一致性确认。请重新读取结果，不要重复首次开通。'
        return
      }

      readbackState.value = 'verified'
      statusMessage.value = '首次开通已完成，并已从最终权益结果确认目标套餐能力。'
      onRefresh()
    } catch {
      readbackState.value = 'failed'
      errorMessage.value = '首次开通回执已保留，但最终权益读取失败。请重新读取结果，不要重复提交。'
    }
  }

  async function refreshResult() {
    if (!preview.value) return
    pending.value = true
    errorMessage.value = ''
    try {
      const latest = await getSubscriptionChangeReceipt(tenantId(), preview.value.changeId)
      receipt.value = latest

      if (latest.status === 'APPLIED') {
        await verifyReadback()
        return
      }
      if (latest.provisioningTaskId) {
        task.value = await getProvisioningTask(tenantId(), latest.provisioningTaskId)
        readbackState.value = 'pending'
        if (task.value.state === 'APPLIED') {
          receipt.value = await getSubscriptionChangeReceipt(tenantId(), preview.value.changeId)
          if (receipt.value.status === 'APPLIED') await verifyReadback()
        }
        return
      }
      readbackState.value = 'pending'
      statusMessage.value = '当前结果仍未形成最终权益事实。'
    } catch (error) {
      errorMessage.value = describeError(error, '结果读取失败。')
    } finally {
      pending.value = false
    }
  }

  async function retryTask() {
    if (!task.value?.retryAllowed || !receipt.value?.provisioningTaskId) return
    pending.value = true
    errorMessage.value = ''
    try {
      task.value = await retryProvisioningTask(tenantId(), receipt.value.provisioningTaskId, {
        requestId: commercialRequestId('platform-initial-retry'),
        expectedRevision: task.value.revision,
        reason: '继续首次开通准备任务',
      })
      statusMessage.value = '已恢复原准备任务；不会创建第二份首次订阅。'
    } catch (error) {
      if (error instanceof CommercialApiError && error.code === 'conflict') {
        await refreshResult()
      } else {
        errorMessage.value = describeError(error, '准备任务恢复失败。')
      }
    } finally {
      pending.value = false
    }
  }

  async function recoverConcurrentActivation() {
    try {
      const existing = await getTenantSubscription(tenantId())
      if (existing?.subscriptionId) {
        statusMessage.value = '该租户已被其他管理员完成或接管首次开通，正在切换到当前真实订阅。'
        onRefresh()
        return
      }
    } catch {
      // Preserve the conflict if authoritative readback is unavailable.
    }
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

  onMounted(loadPlanCatalog)

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
    readbackState,
    loadingPlans,
    loadingTargets,
    pending,
    errorMessage,
    statusMessage,
    selectedVersion,
    eligibleCandidates,
    provisioningStatus,
    receiptTone,
    versionKey,
    formatValidity,
    limitLabel,
    loadPublishedVersions,
    createPreview,
    confirmPreview,
    refreshResult,
    retryTask,
  }
}
