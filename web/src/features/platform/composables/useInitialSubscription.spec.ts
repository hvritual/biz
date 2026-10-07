import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, ref, type EffectScope } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { useInitialSubscription } from './useInitialSubscription'
import { initialSubscriptionReadbackMatches } from '@/services/commercial/initialSubscriptionReadback'
import * as api from '@/services/commercial/platformCommercial'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'

vi.mock('@/services/commercial/platformCommercial', async (original) => {
  const actual = await original<typeof import('@/services/commercial/platformCommercial')>()
  return { ...actual, listPlans: vi.fn(), listPlanVersions: vi.fn(), checkPlanEligibility: vi.fn(),
    previewSubscriptionChange: vi.fn(), confirmSubscriptionChange: vi.fn(), getTenantSubscription: vi.fn(),
    explainTenantEntitlements: vi.fn(), getSubscriptionChangeReceipt: vi.fn(), getProvisioningTask: vi.fn(),
    retryProvisioningTask: vi.fn(), commercialRequestId: vi.fn(() => 'request-first') }
})
// This suite isolates orchestration. The actual readback policy has its own
// positive/negative matrix in initialSubscriptionReadback.spec.ts.
vi.mock('@/services/commercial/initialSubscriptionReadback', () => ({ initialSubscriptionReadbackMatches: vi.fn() }))
vi.mock('@/services/runtime/sessionCoordinator', () => ({ subscribeSessionContextChange: vi.fn(() => vi.fn()) }))

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
