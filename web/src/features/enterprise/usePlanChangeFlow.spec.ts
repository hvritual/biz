import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick, reactive } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { usePlanChangeFlow } from './usePlanChangeFlow'
import { confirmMySubscriptionChange, getMySubscriptionChangeReceipt, previewMySubscriptionChange } from '@/services/enterprise/planChangeRuntime'
import type { SubscriptionChangePreviewDTO, SubscriptionChangeReceiptDTO, TenantSubscriptionDTO } from '@/services/commercial/platformCommercial'

vi.mock('@/services/enterprise/planChangeRuntime', async (original) => ({
  ...await original<typeof import('@/services/enterprise/planChangeRuntime')>(),
  previewMySubscriptionChange: vi.fn(), confirmMySubscriptionChange: vi.fn(),
  getMySubscriptionChangeReceipt: vi.fn(),
}))
let wrapper: VueWrapper | undefined
const basePreview = { tenantId: 'A', changeId: 'change-1', action: 'RENEW', previewHash: 'hash-1', pricingBasis: 'NO_PRICE_REFERENCE', quotaValidationRequired: false } as SubscriptionChangePreviewDTO
const baseReceipt = { tenantId: 'A', changeId: 'change-1', action: 'RENEW', status: 'APPLIED' } as SubscriptionChangeReceiptDTO

function setup() {
  const props = reactive({
    session: { authenticated: true, actor_kind: 'tenant', user_id: 'u1', active_tenant_id: 'A', context_version: 1 },
    subscription: { tenantId: 'A', pendingChangeId: '' } as TenantSubscriptionDTO,
    blocked: false,
  })
  let flow!: ReturnType<typeof usePlanChangeFlow>
  const changed = vi.fn()
  wrapper = mount(defineComponent({ setup() { flow = usePlanChangeFlow(props, changed); return () => null } }))
  return { props, flow, changed }
}
afterEach(() => { wrapper?.unmount(); vi.resetAllMocks() })

describe('plan lifecycle read-only recovery boundary', () => {
  it('blocks both preview and confirmation when current access cannot be verified', async () => {
    const { props, flow } = setup()
    flow.action.value = 'RENEW'
    await nextTick()
    props.blocked = true
    flow.preview.value = basePreview
    await flow.createPreview()
    await flow.confirmPreview()
    expect(previewMySubscriptionChange).not.toHaveBeenCalled()
    expect(confirmMySubscriptionChange).not.toHaveBeenCalled()
  })
  it('queries the original operation after an uncertain confirmation without repeating the write', async () => {
    vi.mocked(previewMySubscriptionChange).mockResolvedValue(basePreview)
    vi.mocked(confirmMySubscriptionChange).mockRejectedValue(new Error('connection lost'))
    vi.mocked(getMySubscriptionChangeReceipt).mockResolvedValue(baseReceipt)
    const { flow } = setup()
    flow.action.value = 'RENEW'
    await nextTick()
    await flow.createPreview()
    await flow.confirmPreview()
    expect(flow.confirmationUnknown.value).toBe(true)
    flow.resetLifecycle()
    expect(flow.preview.value?.changeId).toBe('change-1')
    await flow.confirmPreview()
    expect(confirmMySubscriptionChange).toHaveBeenCalledTimes(1)
    await flow.refreshReceipt()
    expect(getMySubscriptionChangeReceipt).toHaveBeenCalledWith(expect.objectContaining({ active_tenant_id: 'A' }), 'change-1')
    expect(flow.receipt.value?.status).toBe('APPLIED')
    expect(flow.confirmationUnknown.value).toBe(false)
  })
  it('drops an old preview when the session changes during the read', async () => {
    let resolve!: (value: SubscriptionChangePreviewDTO) => void
    vi.mocked(previewMySubscriptionChange).mockImplementation(() => new Promise((done) => { resolve = done }))
    const { props, flow } = setup()
    flow.action.value = 'RENEW'
    await nextTick()
    const task = flow.createPreview()
    props.session.context_version = 2
    resolve(basePreview)
    await task
    expect(flow.preview.value).toBeNull()
    expect(flow.receipt.value).toBeNull()
  })
})
