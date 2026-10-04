import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { setUiLocale } from '@/i18n'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import { loadEnterprisePlanReadModel, type EnterprisePlanReadModel } from '@/services/enterprise/planRuntime'
import { useEnterprisePlanStore } from './enterprisePlan'

const session = (tenant = 'A', version = 1) => ({ authenticated: true, actor_kind: 'tenant', user_id: 'user', active_tenant_id: tenant, context_version: version })
const enterprise = reactive({ sourceKind: 'api', tenantId: 'A', session: session(), members: [] as { status: string }[], audit: vi.fn() })
vi.mock('@/stores/enterprise', () => ({ useEnterpriseStore: () => enterprise }))
vi.mock('@/services/enterprise/planRuntime', async (original) => ({ ...await original<typeof import('@/services/enterprise/planRuntime')>(), loadEnterprisePlanReadModel: vi.fn() }))
const loader = vi.mocked(loadEnterprisePlanReadModel)
let pinia: ReturnType<typeof createPinia>

function model(tenant = 'A'): EnterprisePlanReadModel {
  return {
    session: session(tenant),
    subscription: { tenantId: tenant, planCode: 'rental-growth-2026', state: 'active', periodEnd: '2027-01-01T00:00:00Z' },
    entitlements: { tenantId: tenant, decisions: [
      { kind: 'module', moduleCode: 'access-management', key: 'access-management', allowed: true },
      { kind: 'quota', moduleCode: 'access-management', key: 'tenant.members', allowed: true, limit: { value: 20, unlimited: false } },
    ] },
    usage: { usages: [{ moduleCode: 'access-management', key: 'tenant.members', known: true, used: 18, evidence: 'fixture' }] },
    usageError: '',
  } as EnterprisePlanReadModel
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  setUiLocale('zh-CN')
  enterprise.sourceKind = 'api'
  enterprise.tenantId = 'A'
  enterprise.session = session()
  loader.mockReset().mockResolvedValue(model())
})
afterEach(() => disposePinia(pinia))

describe('enterprise plan read recovery', () => {
  it('never returns a demo plan or quota while an API read is pending or has failed', async () => {
    loader.mockRejectedValue(new Error('private transport detail'))
    const store = useEnterprisePlanStore()
    expect(store.features).toEqual([])
    expect(store.quotas).toEqual([])
    expect(store.currentPlan).not.toBe('标准版')
    await store.load()
    expect(store.model).toBeNull()
    expect(store.readIssue).toBe('unavailable')
    expect(store.error).not.toContain('private transport detail')
    expect(store.quotas).toEqual([])
  })
  it('keeps a previous same-scope snapshot explicitly stale and blocks new changes', async () => {
    const store = useEnterprisePlanStore()
    await store.load()
    expect(store.canUseCurrentFacts).toBe(true)
    loader.mockRejectedValueOnce(new Error('offline'))
    await store.load()
    expect(store.model?.subscription.tenantId).toBe('A')
    expect(store.stale).toBe(true)
    expect(store.canUseCurrentFacts).toBe(false)
    await store.load()
    expect(store.stale).toBe(false)
    expect(store.canUseCurrentFacts).toBe(true)
  })
  it.each([401, 403, 409])('removes cached protected data for HTTP %i', async (status) => {
    const store = useEnterprisePlanStore()
    await store.load()
    loader.mockRejectedValueOnce(new CommercialApiError('rejected', status, status === 401 ? 'unauthenticated' : status === 403 ? 'forbidden' : 'conflict'))
    await store.load()
    expect(store.model).toBeNull()
    expect(store.lastReadAt).toBe('')
    expect(store.features).toEqual([])
    expect(store.quotas).toEqual([])
    expect(store.canUseCurrentFacts).toBe(false)
  })
  it('preserves limits but not fabricated usage when usage alone fails', async () => {
    loader.mockResolvedValue({ ...model(), usage: { usages: [] }, usageError: 'unavailable' })
    const store = useEnterprisePlanStore()
    await store.load()
    expect(store.quotas[0]?.total).toBe(20)
    expect(store.quotas[0]?.used).toBeNull()
    expect(store.quotas[0]?.status).toBe('用量未知')
    expect(store.readIssue).toBe('usage')
    expect(store.canUseCurrentFacts).toBe(false)
  })
  it('clears the old tenant synchronously and ignores its delayed reply', async () => {
    let resolve!: (value: EnterprisePlanReadModel) => void
    loader.mockImplementationOnce(() => new Promise((done) => { resolve = done }))
    const store = useEnterprisePlanStore()
    const old = store.load()
    await Promise.resolve()
    enterprise.tenantId = 'B'
    enterprise.session = session('B')
    loader.mockResolvedValue(model('B'))
    expect(store.model).toBeNull()
    await store.load()
    resolve(model('A'))
    await old
    expect(store.model?.subscription.tenantId).toBe('B')
  })
  it('refreshes after a same-tenant session-version change', async () => {
    const store = useEnterprisePlanStore()
    await store.load()
    enterprise.session = session('A', 2)
    expect(store.model).toBeNull()
    loader.mockResolvedValue({ ...model(), session: session('A', 2) })
    await store.load()
    expect(store.model?.session.context_version).toBe(2)
  })
})
