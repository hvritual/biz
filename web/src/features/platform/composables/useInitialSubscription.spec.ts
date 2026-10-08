import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, ref, type EffectScope } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { useInitialSubscription } from './useInitialSubscription'
import { initialSubscriptionReadbackMatches } from '@/services/commercial/initialSubscriptionOutcome'
import * as api from '@/services/commercial/platformCommercial'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import { recallInitialSubscription, rememberInitialSubscription, forgetInitialSubscription } from '@/services/commercial/initialSubscriptionJournal'

vi.mock('@/services/commercial/platformCommercial', async (original) => {
  const actual = await original<typeof import('@/services/commercial/platformCommercial')>()
  return { ...actual, listPlans: vi.fn(), listPlanVersions: vi.fn(), getPlanVersion: vi.fn(), checkPlanEligibility: vi.fn(),
    previewSubscriptionChange: vi.fn(), confirmSubscriptionChange: vi.fn(), getTenantSubscription: vi.fn(),
    explainTenantEntitlements: vi.fn(), getSubscriptionChangeReceipt: vi.fn(), getSubscriptionChangePreview: vi.fn(), getProvisioningTask: vi.fn(),
    retryProvisioningTask: vi.fn(), commercialRequestId: vi.fn(() => 'request-first') }
})
// This suite isolates orchestration. The actual readback policy has its own
// positive/negative matrix in initialSubscriptionReadback.spec.ts.
vi.mock('@/services/commercial/initialSubscriptionOutcome', () => ({ initialSubscriptionReadbackMatches: vi.fn() }))
vi.mock('@/services/runtime/sessionCoordinator', () => ({ subscribeSessionContextChange: vi.fn(() => vi.fn()) }))
vi.mock('@/services/commercial/initialSubscriptionJournal', () => ({
  rememberInitialSubscription: vi.fn(), recallInitialSubscription: vi.fn(), forgetInitialSubscription: vi.fn(async () => {}),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

const target = { planCode: 'office-pro', version: '3', state: 'PUBLISHED', name: 'Office Pro' } as api.PlanVersionDTO
const proposed = { changeId: 'change-first', tenantId: 'tenant-a', action: 'INITIAL', previewHash: 'b'.repeat(64), target } as api.SubscriptionChangePreviewDTO
const applied = { changeId: 'change-first', tenantId: 'tenant-a', action: 'INITIAL', previewHash: 'b'.repeat(64), status: 'APPLIED' } as api.SubscriptionChangeReceiptDTO
const scopes: EffectScope[] = []

function setup() {
  const tenant = ref('tenant-a')
  const refreshed = vi.fn()
  const scope = effectScope()
  scopes.push(scope)
  const flow = scope.run(() => useInitialSubscription(() => tenant.value, refreshed))
  if (!flow) throw new Error('Missing active scope')
  flow.salesScope.value = 'office'
  flow.planCode.value = 'office-pro'
  flow.candidates.value = [{ version: target, eligible: true, reason: '' }]
  flow.selectedVersionKey.value = 'office-pro:3'
  flow.preview.value = { ...proposed }
  flow.approved.value = true
  flow.confirmReason.value = 'approve'
  return { flow, tenant, refreshed, scope }
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(subscribeSessionContextChange).mockReturnValue(vi.fn())
  vi.mocked(initialSubscriptionReadbackMatches).mockReturnValue(true)
  vi.mocked(rememberInitialSubscription).mockResolvedValue(true)
  vi.mocked(recallInitialSubscription).mockResolvedValue(null)
  vi.mocked(forgetInitialSubscription).mockResolvedValue(undefined)
  vi.mocked(api.getSubscriptionChangePreview).mockResolvedValue(proposed)
  vi.mocked(api.getPlanVersion).mockResolvedValue(target)
  vi.mocked(api.commercialRequestId).mockReturnValue('request-first')
  vi.mocked(api.getTenantSubscription).mockResolvedValue({ tenantId: 'tenant-a' } as api.TenantSubscriptionDTO)
  vi.mocked(api.explainTenantEntitlements).mockResolvedValue({ tenantId: 'tenant-a' } as api.EntitlementView)
})
afterEach(() => { for (const scope of scopes.splice(0)) scope.stop() })

describe('first subscription in-flight context and confirmation recovery', () => {
  it('submits once even when invoked twice before rendering can disable the button', async () => {
    const { flow } = setup()
    const response = deferred<api.SubscriptionChangeReceiptDTO>()
    vi.mocked(api.confirmSubscriptionChange).mockReturnValue(response.promise)
    const first = flow.confirmPreview()
    await flow.confirmPreview()
    await flushPromises()
    expect(api.confirmSubscriptionChange).toHaveBeenCalledTimes(1)
    response.resolve(applied)
    await first
    expect(flow.verificationState.value).toBe('verified')
    await flow.confirmPreview()
    expect(api.confirmSubscriptionChange).toHaveBeenCalledTimes(1)
  })

  it('keeps a lost receipt locked and recovers with a read, not a second confirmation', async () => {
    const { flow, refreshed } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockRejectedValue(new Error('network timeout'))
    await flow.confirmPreview()
    expect(flow.confirmationSubmitted.value).toBe(true)
    expect(flow.preview.value?.changeId).toBe('change-first')
    await flow.confirmPreview()
    expect(api.confirmSubscriptionChange).toHaveBeenCalledTimes(1)
    vi.mocked(api.getSubscriptionChangeReceipt).mockResolvedValue(applied)
    await flow.refreshResult()
    expect(api.getSubscriptionChangeReceipt).toHaveBeenCalledWith('tenant-a', 'change-first')
    expect(refreshed).toHaveBeenCalledTimes(1)
    expect(api.confirmSubscriptionChange).toHaveBeenCalledTimes(1)
  })

  it('permits fresh preview only after both original receipt and tenant subscription are proven absent', async () => {
    const { flow } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockRejectedValue(new api.CommercialApiError('stale preview', 409, 'conflict'))
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    vi.mocked(api.getTenantSubscription).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    await flow.confirmPreview()
    expect(flow.confirmationSubmitted.value).toBe(false)
    expect(flow.preview.value).toBeNull()
    expect(flow.selectedVersionKey.value).toBe('')
    expect(flow.recoveryChangeId.value).toBe('')
    expect(flow.statusMessage.value).toContain('重新检查版本')
    expect(api.getSubscriptionChangeReceipt).toHaveBeenCalledWith('tenant-a', 'change-first')
    expect(api.getTenantSubscription).toHaveBeenCalledWith('tenant-a')
  })

  it('keeps the original request locked when a conflict cannot be authoritatively reconciled', async () => {
    const { flow } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockRejectedValue(new api.CommercialApiError('conflict', 409, 'conflict'))
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new Error('receipt read unavailable'))
    await flow.confirmPreview()
    expect(flow.confirmationSubmitted.value).toBe(true)
    expect(flow.preview.value?.changeId).toBe('change-first')
    expect(flow.errorMessage.value).toContain('暂不能重复开通')
    expect(api.getTenantSubscription).not.toHaveBeenCalled()
  })

  it('never previews or confirms a price-referenced version even when eligibility reports yes', async () => {
    const { flow } = setup()
    const priced = { ...target, terms: { modules: [], salesScope: ['office'], validityMode: 'fixed_days', validityDays: 365, priceRef: 'external-price' } } as api.PlanVersionDTO
    vi.mocked(api.listPlanVersions).mockResolvedValue({ versions: [priced], nextAfterVersion: '' })
    vi.mocked(api.checkPlanEligibility).mockResolvedValue({ eligible: true, reason: '' })
    await flow.loadPublishedVersions()
    expect(flow.candidates.value).toHaveLength(1)
    expect(flow.candidates.value[0]?.eligible).toBe(false)
    expect(flow.candidates.value[0]?.reason).toContain('外部商业审批')
    expect(flow.selectedVersion.value).toBeNull()
    await flow.createPreview()
    expect(api.previewSubscriptionChange).not.toHaveBeenCalled()
    flow.candidates.value = [{ version: priced, eligible: true, reason: '' }]
    flow.selectedVersionKey.value = 'office-pro:3'
    flow.preview.value = { ...proposed, target: priced }
    flow.approved.value = true
    flow.confirmReason.value = 'approve'
    await flow.confirmPreview()
    expect(api.confirmSubscriptionChange).not.toHaveBeenCalled()
    expect(flow.errorMessage.value).toContain('外部商业审批')
  })

  it.each(['preview', 'confirm'])('rejects over-512-byte %s reason before any write or flow lock', async (stage) => {
    const { flow } = setup()
    const oversized = '授'.repeat(172) // UTF-8 bytes > 512; Go validates byte length.
    if (stage === 'preview') {
      flow.previewReason.value = oversized
      await flow.createPreview()
      expect(api.previewSubscriptionChange).not.toHaveBeenCalled()
      expect(flow.confirmationSubmitted.value).toBe(false)
    } else {
      flow.confirmReason.value = oversized
      await flow.confirmPreview()
      expect(api.confirmSubscriptionChange).not.toHaveBeenCalled()
      expect(flow.confirmationSubmitted.value).toBe(false)
    }
    expect(flow.errorMessage.value).toContain('512 字节')
  })

  it('recovers a definitive HTTP400 only when original receipt and tenant subscription are both absent', async () => {
    const { flow } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockRejectedValue(new api.CommercialApiError('invalid reason', 400, 'http'))
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    vi.mocked(api.getTenantSubscription).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    await flow.confirmPreview()
    expect(flow.confirmationSubmitted.value).toBe(false)
    expect(flow.preview.value).toBeNull()
    expect(flow.statusMessage.value).toContain('重新检查版本')
    expect(api.getSubscriptionChangeReceipt).toHaveBeenCalledWith('tenant-a', 'change-first')
    expect(api.getTenantSubscription).toHaveBeenCalledWith('tenant-a')
  })

  it('does not unlock an HTTP400 with an unknown original receipt or subscription', async () => {
    const { flow } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockRejectedValue(new api.CommercialApiError('validation', 400, 'http'))
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new Error('read unavailable'))
    await flow.confirmPreview()
    expect(flow.confirmationSubmitted.value).toBe(true)
    expect(flow.preview.value?.changeId).toBe('change-first')
    expect(flow.errorMessage.value).toContain('暂不能重复开通')
  })

  it('does not publish a confirmation received after switching tenants', async () => {
    const { flow, tenant, refreshed } = setup()
    const response = deferred<api.SubscriptionChangeReceiptDTO>()
    vi.mocked(api.confirmSubscriptionChange).mockReturnValue(response.promise)
    const pending = flow.confirmPreview()
    tenant.value = 'tenant-b'
    response.resolve(applied)
    await pending
    expect(flow.receipt.value).toBeNull()
    expect(api.getTenantSubscription).not.toHaveBeenCalled()
    expect(refreshed).not.toHaveBeenCalled()
  })

  it.each(['tenant', 'dispose'])('ignores final readback after %s context invalidation', async (invalidation) => {
    const { flow, tenant, scope, refreshed } = setup()
    const response = deferred<api.EntitlementView>()
    vi.mocked(api.confirmSubscriptionChange).mockResolvedValue(applied)
    vi.mocked(api.explainTenantEntitlements).mockReturnValue(response.promise)
    const pending = flow.confirmPreview()
    await flushPromises()
    expect(api.explainTenantEntitlements).toHaveBeenCalledWith('tenant-a', [])
    if (invalidation === 'tenant') tenant.value = 'tenant-b'
    else scope.stop()
    response.resolve({ tenantId: 'tenant-a' } as api.EntitlementView)
    await pending
    expect(flow.verificationState.value).not.toBe('verified')
    expect(flow.finalSubscription.value).toBeNull()
    expect(refreshed).not.toHaveBeenCalled()
  })

  it('does not surface an old readback failure in a new tenant context', async () => {
    const { flow, tenant } = setup()
    const response = deferred<api.EntitlementView>()
    vi.mocked(api.confirmSubscriptionChange).mockResolvedValue(applied)
    vi.mocked(api.explainTenantEntitlements).mockReturnValue(response.promise)
    const pending = flow.confirmPreview()
    await flushPromises()
    tenant.value = 'tenant-b'
    response.reject(new Error('old tenant failed'))
    await pending
    expect(flow.errorMessage.value).toBe('')
    expect(flow.verificationState.value).toBe('idle')
  })

  it('does not confuse a receipt with a confirmed entitlement decision', async () => {
    const { flow, refreshed } = setup()
    vi.mocked(api.confirmSubscriptionChange).mockResolvedValue(applied)
    vi.mocked(initialSubscriptionReadbackMatches).mockReturnValue(false)
    await flow.confirmPreview()
    expect(flow.receipt.value?.status).toBe('APPLIED')
    expect(flow.verificationState.value).toBe('failed')
    expect(flow.finalSubscription.value).toBeNull()
    expect(refreshed).not.toHaveBeenCalled()
  })

  it('discards target eligibility that arrives after the tenant changes', async () => {
    const { flow, tenant } = setup()
    const response = deferred<api.PlanEligibilityDTO>()
    vi.mocked(api.listPlanVersions).mockResolvedValue({ versions: [target], nextAfterVersion: '' })
    vi.mocked(api.checkPlanEligibility).mockReturnValue(response.promise)
    const pending = flow.loadPublishedVersions()
    await flushPromises()
    tenant.value = 'tenant-b'
    response.resolve({ eligible: true, reason: '', version: target })
    await pending
    expect(flow.candidates.value).toEqual([])
    expect(flow.selectedVersionKey.value).toBe('')
    expect(flow.loadingTargets.value).toBe(false)
  })

  it('does not install a preparation task from a departed tenant', async () => {
    const { flow, tenant } = setup()
    const response = deferred<api.ProvisioningTaskDTO>()
    vi.mocked(api.confirmSubscriptionChange).mockResolvedValue({ ...applied, status: 'PROVISIONING', provisioningTaskId: 'task-first' })
    vi.mocked(api.getProvisioningTask).mockReturnValue(response.promise)
    const pending = flow.confirmPreview()
    await flushPromises()
    tenant.value = 'tenant-b'
    response.resolve({ taskId: 'task-first', tenantId: 'tenant-a', state: 'QUEUED' } as api.ProvisioningTaskDTO)
    await pending
    expect(flow.task.value).toBeNull()
    expect(flow.receipt.value).toBeNull()
    expect(flow.statusMessage.value).toBe('')
  })
})


describe('first subscription reload recovery', () => {
  it('restores an applied confirmed change from tenant receipt without reading an actor-private preview', async () => {
    const { flow, refreshed } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    vi.mocked(api.getSubscriptionChangeReceipt).mockResolvedValue({ ...applied, after: {
      tenantId: 'tenant-a', subscriptionId: 'sub-first', kind: 'BASE', state: 'ACTIVE',
      planCode: 'office-pro', planVersion: '3', salesScope: 'office',
    } as api.TenantSubscriptionDTO })
    await flow.restoreSubmission()
    expect(flow.verificationState.value).toBe('verified')
    expect(refreshed).toHaveBeenCalledOnce()
    expect(api.getPlanVersion).toHaveBeenCalledWith('office-pro', '3')
    expect(api.getSubscriptionChangePreview).not.toHaveBeenCalled()
    expect(api.confirmSubscriptionChange).not.toHaveBeenCalled()
  })

  it('allows another authorized administrator to recover and retry the same confirmed provisioning task', async () => {
    const { flow } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    vi.mocked(api.getSubscriptionChangeReceipt).mockResolvedValue({
      ...applied, status: 'PROVISIONING', provisioningTaskId: 'task-first',
      after: { tenantId: 'tenant-a', subscriptionId: 'sub-first', kind: 'BASE', state: 'PROVISIONING',
        planCode: 'office-pro', planVersion: '3', salesScope: 'office', pendingChangeId: 'change-first' } as api.TenantSubscriptionDTO,
    })
    vi.mocked(api.getProvisioningTask).mockResolvedValue({
      taskId: 'task-first', tenantId: 'tenant-a', changeId: 'change-first', state: 'FAILED', revision: '7', retryAllowed: true,
    } as api.ProvisioningTaskDTO)
    vi.mocked(api.retryProvisioningTask).mockResolvedValue({
      taskId: 'task-first', tenantId: 'tenant-a', state: 'RETRY_WAIT', revision: '8', retryAllowed: false,
    } as api.ProvisioningTaskDTO)
    await flow.restoreSubmission()
    expect(api.getSubscriptionChangePreview).not.toHaveBeenCalled()
    expect(api.getPlanVersion).toHaveBeenCalledWith('office-pro', '3')
    expect(flow.task.value?.state).toBe('FAILED')
    expect(flow.task.value?.retryAllowed).toBe(true)
    expect(flow.confirmationSubmitted.value).toBe(true)
    await flow.retryTask()
    expect(api.retryProvisioningTask).toHaveBeenCalledWith('tenant-a', 'task-first', expect.objectContaining({ expectedRevision: '7' }))
    expect(api.confirmSubscriptionChange).not.toHaveBeenCalled()
  })

  it('rejects a forged or cross-tenant confirmed receipt without fetching a private preview', async () => {
    const { flow } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    vi.mocked(api.getSubscriptionChangeReceipt).mockResolvedValue({
      ...applied, after: { tenantId: 'tenant-b', subscriptionId: 'sub-first', kind: 'BASE',
        planCode: 'office-pro', planVersion: '3', salesScope: 'office' } as api.TenantSubscriptionDTO,
    })
    await flow.restoreSubmission()
    expect(flow.verificationState.value).not.toBe('verified')
    expect(api.getPlanVersion).not.toHaveBeenCalled()
    expect(api.getSubscriptionChangePreview).not.toHaveBeenCalled()
  })

  it('does not restore a change belonging to another tenant', async () => {
    const { flow, refreshed } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    vi.mocked(api.getSubscriptionChangePreview).mockResolvedValue({ ...proposed, tenantId: 'tenant-b' })
    await flow.restoreSubmission()
    expect(flow.verificationState.value).not.toBe('verified')
    expect(refreshed).not.toHaveBeenCalled()
    expect(api.getSubscriptionChangeReceipt).toHaveBeenCalledOnce()
  })

  it('retries a lost confirmation with the saved request id, hash and reason, not a new request', async () => {
    const { flow } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    const saved = { tenantId: 'tenant-a', changeId: 'change-first', salesScope: 'office',
      input: { requestId: 'original-request', previewHash: proposed.previewHash, reason: 'original reason' } }
    vi.mocked(recallInitialSubscription).mockResolvedValue(saved)
    const absent = new api.CommercialApiError('not found', 404, 'http')
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValueOnce(absent).mockRejectedValueOnce(absent).mockResolvedValue(applied)
    vi.mocked(api.confirmSubscriptionChange).mockResolvedValue(applied)
    await flow.restoreSubmission()
    expect(flow.retryConfirmationAllowed.value).toBe(true)
    await flow.retryOriginalConfirmation()
    expect(api.confirmSubscriptionChange).toHaveBeenCalledTimes(1)
    expect(api.confirmSubscriptionChange).toHaveBeenCalledWith('tenant-a', 'change-first', saved.input)
  })

  it('ignores original-task restore after tenant change', async () => {
    const { flow, tenant, refreshed } = setup()
    flow.preview.value = null
    flow.recoveryChangeId.value = 'change-first'
    const response = deferred<api.SubscriptionChangePreviewDTO>()
    vi.mocked(api.getSubscriptionChangeReceipt).mockRejectedValue(new api.CommercialApiError('not found', 404, 'http'))
    vi.mocked(api.getSubscriptionChangePreview).mockReturnValue(response.promise)
    const pending = flow.restoreSubmission()
    tenant.value = 'tenant-b'
    response.resolve(proposed)
    await pending
    expect(flow.preview.value).toBeNull()
    expect(flow.receipt.value).toBeNull()
    expect(refreshed).not.toHaveBeenCalled()
  })
})
