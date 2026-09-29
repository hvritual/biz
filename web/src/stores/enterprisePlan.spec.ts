import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, reactive, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

const tenantId = ref('tenant-a')
const load = vi.fn()
const enterprise = reactive({ tenantId, sourceKind: 'api' as const, members: [], audit: vi.fn() })

vi.mock('@/stores/enterprise', () => ({
  useEnterpriseStore: () => enterprise,
}))
vi.mock('@/services/enterprise/planRuntime', () => ({
  enterprisePlanRuntimeError: () => '套餐与权益暂不可用，请稍后重试。',
  loadEnterprisePlanReadModel: (...args: unknown[]) => load(...args),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function model(id: string) {
  return {
    session: { authenticated: true, actor_kind: 'tenant', active_tenant_id: id, context_version: 1 },
    subscription: { tenantId: id, planCode: 'rental-growth-2026', state: 'ACTIVE', periodStart: '', periodEnd: '', createdAt: '' },
    entitlements: { tenantId: id, decisions: [] },
    usage: { usages: [] },
    usageError: '',
  }
}

describe('enterprise plan tenant isolation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    tenantId.value = 'tenant-a'
    load.mockReset()
  })

  it('clears stale plan state and rejects a late response after a tenant change', async () => {
    const first = deferred<ReturnType<typeof model>>()
    const second = deferred<ReturnType<typeof model>>()
    load.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const { useEnterprisePlanStore } = await import('./enterprisePlan')
    const store = useEnterprisePlanStore()
    await nextTick()
    tenantId.value = 'tenant-b'
    await nextTick()
    expect(store.model).toBeNull()
    second.resolve(model('tenant-b'))
    await vi.waitFor(() => expect(store.model?.subscription.tenantId).toBe('tenant-b'))
    first.resolve(model('tenant-a'))
    await vi.waitFor(() => expect(store.model?.subscription.tenantId).toBe('tenant-b'))
  })
})
