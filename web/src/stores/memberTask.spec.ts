import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createSeed } from '@/services/demo/seed'
import type { TrustedSession } from '@/services/runtime/api'
import type { EnterpriseSourceState } from '@/services/enterprise/dataSource'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'
import { CommercialApiError } from '@/services/commercial/platformCommercial'

// These are controlled API doubles, not evidence of real backend acceptance.
const remote = vi.hoisted(() => ({
  initial: vi.fn(), load: vi.fn(), switchTenant: vi.fn(), session: vi.fn(),
  query: vi.fn(), removed: vi.fn(), suspend: vi.fn(), activate: vi.fn(), get: vi.fn(),
}))
vi.mock('@/services/enterprise/dataSource', async (original) => ({
  ...await original<typeof import('@/services/enterprise/dataSource')>(),
  createEnterpriseDataSource: () => ({ kind: 'api', initial: remote.initial, load: remote.load, switchTenant: remote.switchTenant, persist: vi.fn() }),
}))
vi.mock('@/services/enterprise/memberRuntime', async (original) => ({
  ...await original<typeof import('@/services/enterprise/memberRuntime')>(),
  readEnterpriseMemberSession: remote.session, queryEnterpriseMembers: remote.query,
  listRemovedEnterpriseMembers: remote.removed, suspendEnterpriseMember: remote.suspend,
  activateEnterpriseMember: remote.activate, getEnterpriseMember: remote.get,
}))
vi.mock('@/services/runtime/sessionCoordinator', () => ({
  subscribeSessionContextChange: () => () => undefined, publishSessionContextChange: vi.fn(),
}))
vi.mock('@/services/runtime/authorization', () => ({
  authorizationApiMode: () => true, currentAuthorizationAllows: () => true,
  currentAuthorizationState: { status: 'idle', snapshot: null },
  ensureCurrentAuthorization: vi.fn(), invalidateCurrentAuthorization: vi.fn(),
}))
import { MemberStatusMutationError, memberMutationNeedsInspection, useEnterpriseStore } from './enterprise'

function trusted(tenant = 'shanghai', version = 1): TrustedSession {
  return { authenticated: true, actor_kind: 'tenant', user_id: 'viewer-1', active_tenant_id: tenant, context_version: version }
}
function source(tenant = 'shanghai', version = 1): EnterpriseSourceState {
  return { tenantId: tenant, snapshot: createSeed(tenant), session: trusted(tenant, version), loadedDomains: ['members', 'roles', 'departments'] }
}
function member(userId = 'member-2', version = 2, status = 'TENANT_MEMBER_STATUS_SUSPENDED'): EnterpriseTenantMember {
  return { userId, version, status, username: 'test.member', email: 'synthetic@example.test', name: userId,
    phone: '', employeeId: '', position: '', departmentId: '', roles: [], derivedDataScope: 'none' }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const query = (value = '') => ({ query: value, roleId: '', departmentId: '', status: '' as const, page: 1, pageSize: 10 })
const page = (id: string) => ({ members: [member(id)], total: 1 })
const turn = async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() }

beforeEach(() => {
  vi.resetAllMocks()
  localStorage.clear()
  setActivePinia(createPinia())
  remote.initial.mockImplementation(() => source())
  remote.load.mockImplementation(async () => source())
  remote.switchTenant.mockImplementation(async (tenant: string) => source(tenant, tenant === 'shanghai' ? 3 : 2))
  remote.session.mockResolvedValue(trusted())
  remote.query.mockResolvedValue(page('latest'))
  remote.removed.mockResolvedValue({ members: [], total: 0 })
  remote.suspend.mockImplementation(async (_session, value) => member(value.userId))
  remote.activate.mockImplementation(async (_session, value) => member(value.userId, 2, 'TENANT_MEMBER_STATUS_ACTIVE'))
  remote.get.mockImplementation(async (_session, id) => member(id))
})

describe('member task query ownership', () => {
  it('starts ownership before the session read, so an older session cannot issue the winning query', async () => {
    const store = useEnterpriseStore(), oldSession = deferred<TrustedSession>()
    remote.session.mockReturnValueOnce(oldSession.promise)
    const old = store.queryMembers(query('old'))
    await expect(store.queryMembers(query('new'))).resolves.toBe(true)
    oldSession.resolve(trusted())
    await expect(old).resolves.toBe(false)
    expect(remote.query).toHaveBeenCalledTimes(1)
    expect(remote.query.mock.calls[0]?.[1].query).toBe('new')
    expect(store.members[0]?.id).toBe('latest')
  })
  it('ignores an out-of-order result without replacing the newest list', async () => {
    const store = useEnterpriseStore(), oldPage = deferred<ReturnType<typeof page>>()
    remote.query.mockReturnValueOnce(oldPage.promise)
    const old = store.queryMembers(query('old'))
    await turn()
    await store.queryMembers(query('new'))
    oldPage.resolve(page('stale'))
    await expect(old).resolves.toBe(false)
    expect(store.members[0]?.id).toBe('latest')
  })
  it('does not let an old error or finally clear a newer loading state', async () => {
    const store = useEnterpriseStore(), oldPage = deferred<ReturnType<typeof page>>(), nextPage = deferred<ReturnType<typeof page>>()
    remote.query.mockReturnValueOnce(oldPage.promise).mockReturnValueOnce(nextPage.promise)
    const old = store.queryMembers(query('old')); await turn()
    const next = store.queryMembers(query('new')); await turn()
    oldPage.reject(new Error('old network failure'))
    await expect(old).resolves.toBe(false)
    expect(store.memberQueryLoading).toBe(true)
    expect(store.memberQueryError).toBe('')
    nextPage.resolve(page('latest'))
    await next
    expect(store.memberQueryLoading).toBe(false)
  })
  it('keeps the current failure observable and permits a read-only retry', async () => {
    const store = useEnterpriseStore()
    remote.query.mockRejectedValueOnce(new Error('current read failed'))
    await expect(store.queryMembers(query('retained'))).rejects.toThrow()
    expect(store.memberQueryError).not.toBe('')
    expect(store.memberQueryLoading).toBe(false)
    await store.queryMembers(query('retained'))
    expect(store.memberQueryError).toBe('')
    expect(remote.suspend).not.toHaveBeenCalled()
    expect(remote.activate).not.toHaveBeenCalled()
  })
  it('rejects the old tenant response even after switching away and back', async () => {
    const store = useEnterpriseStore(), oldPage = deferred<ReturnType<typeof page>>()
    remote.query.mockReturnValueOnce(oldPage.promise)
    const old = store.queryMembers(query('old')); await turn()
    await store.switchTenant('hangzhou')
    await store.switchTenant('shanghai')
    remote.session.mockResolvedValue(trusted('shanghai', 3))
    await store.queryMembers(query('new'))
    oldPage.resolve(page('stale'))
    await expect(old).resolves.toBe(false)
    expect(store.members[0]?.id).toBe('latest')
  })
  it('does not apply a removed-member response across a tenant switch', async () => {
    const store = useEnterpriseStore(), oldPage = deferred<ReturnType<typeof page>>()
    remote.removed.mockReturnValueOnce(oldPage.promise)
    const old = store.queryRemovedMembers(); await turn()
    await store.switchTenant('hangzhou')
    oldPage.resolve(page('old-removed'))
    await expect(old).resolves.toBe(false)
    expect(store.removedMembers).toEqual([])
  })
})

describe('member task state confirmation', () => {
  const targets = [{ id: 'member-2', version: 1 }, { id: 'member-3', version: 1 }]
  it('requires a status and version read for every successful member', async () => {
    const store = useEnterpriseStore()
    await store.changeStatuses(targets, 'suspend', 'synthetic test reason')
    expect(remote.suspend).toHaveBeenCalledTimes(2)
    expect(remote.get.mock.calls.map((call) => call[1])).toEqual(['member-2', 'member-3'])
  })
  it('stops after a mismatched readback rather than reporting the whole batch successful', async () => {
    const store = useEnterpriseStore()
    remote.get.mockResolvedValueOnce(member('member-2', 1, 'TENANT_MEMBER_STATUS_ACTIVE'))
    await expect(store.changeStatuses(targets, 'suspend', 'synthetic test reason')).rejects.toThrow()
    expect(remote.suspend).toHaveBeenCalledTimes(1)
  })
  it('treats failed readback as uncertain, does not resend the first mutation or continue the batch', async () => {
    const store = useEnterpriseStore()
    remote.get.mockRejectedValueOnce(new Error('read timed out'))
    await expect(store.changeStatuses(targets, 'suspend', 'synthetic test reason')).rejects.toThrow('read timed out')
    expect(remote.suspend).toHaveBeenCalledTimes(1)
  })
  it('classifies a session failure before any write as pre_write_failure', async () => {
    const store = useEnterpriseStore()
    remote.session.mockRejectedValueOnce(new Error('session unavailable'))
    const error = await store.changeStatus('member-2', 'suspend', 1, 'synthetic test reason').catch((cause) => cause)
    expect(error).toBeInstanceOf(MemberStatusMutationError)
    expect(error).toMatchObject({ outcome: 'pre_write_failure' })
    expect(memberMutationNeedsInspection(error)).toBe(false)
    expect(remote.suspend).not.toHaveBeenCalled()
    expect(remote.get).not.toHaveBeenCalled()
  })
  it('classifies an explicit 409 write rejection without locking inspection', async () => {
    const store = useEnterpriseStore()
    remote.suspend.mockRejectedValueOnce(new CommercialApiError('mutation conflict', 409, 'conflict'))
    const error = await store.changeStatus('member-2', 'suspend', 1, 'synthetic test reason').catch((cause) => cause)
    expect(error).toBeInstanceOf(MemberStatusMutationError)
    expect(error).toMatchObject({ outcome: 'rejected' })
    expect(memberMutationNeedsInspection(error)).toBe(false)
    expect(remote.suspend).toHaveBeenCalledTimes(1)
    expect(remote.get).not.toHaveBeenCalled()
  })
  it('classifies an unstructured mutation transport failure as write_uncertain', async () => {
    const store = useEnterpriseStore()
    remote.suspend.mockRejectedValueOnce(new TypeError('network failed'))
    const error = await store.changeStatus('member-2', 'suspend', 1, 'synthetic test reason').catch((cause) => cause)
    expect(error).toBeInstanceOf(MemberStatusMutationError)
    expect(error).toMatchObject({ outcome: 'write_uncertain' })
    expect(memberMutationNeedsInspection(error)).toBe(true)
    expect(remote.suspend).toHaveBeenCalledTimes(1)
    expect(remote.get).not.toHaveBeenCalled()
  })
  it('classifies a failed readback after a mutation receipt as write_uncertain', async () => {
    const store = useEnterpriseStore()
    remote.get.mockRejectedValueOnce(new Error('read timed out'))
    const error = await store.changeStatus('member-2', 'suspend', 1, 'synthetic test reason').catch((cause) => cause)
    expect(error).toBeInstanceOf(MemberStatusMutationError)
    expect(error).toMatchObject({ outcome: 'write_uncertain' })
    expect(memberMutationNeedsInspection(error)).toBe(true)
    expect(remote.suspend).toHaveBeenCalledTimes(1)
    expect(remote.get).toHaveBeenCalledTimes(1)
  })
  it('keeps batch certainty per target when a later write is explicitly rejected', async () => {
    const store = useEnterpriseStore()
    remote.suspend
      .mockResolvedValueOnce(member('member-2'))
      .mockRejectedValueOnce(new CommercialApiError('mutation conflict', 409, 'conflict'))
    const error = await store.changeStatuses(targets, 'suspend', 'synthetic test reason').catch((cause) => cause)
    expect(error).toBeInstanceOf(MemberStatusMutationError)
    expect(error).toMatchObject({
      outcome: 'rejected',
      projectionRefreshed: true,
      targets: [
        { id: 'member-2', outcome: 'write_confirmed' },
        { id: 'member-3', outcome: 'rejected' },
      ],
    })
    expect(memberMutationNeedsInspection(error)).toBe(false)
    expect(remote.suspend).toHaveBeenCalledTimes(2)
    expect(remote.get).toHaveBeenCalledTimes(1)
  })
  it('stops between targets when the tenant changes during a mutation', async () => {
    const store = useEnterpriseStore(), receipt = deferred<EnterpriseTenantMember>()
    remote.suspend.mockReturnValueOnce(receipt.promise)
    const operation = store.changeStatuses(targets, 'suspend', 'synthetic test reason'); await turn()
    await store.switchTenant('hangzhou')
    receipt.resolve(member())
    await expect(operation).rejects.toThrow()
    expect(remote.suspend).toHaveBeenCalledTimes(1)
    expect(remote.get).not.toHaveBeenCalled()
  })
  it('also rejects a single-action readback with a different version', async () => {
    const store = useEnterpriseStore()
    remote.get.mockResolvedValueOnce(member('member-2', 99))
    await expect(store.changeStatus('member-2', 'suspend', 1, 'synthetic test reason')).rejects.toThrow()
    expect(remote.suspend).toHaveBeenCalledTimes(1)
  })
  it('inspects every batch target with reads only and distinguishes confirmed from unchanged', async () => {
    const store = useEnterpriseStore()
    remote.get
      .mockResolvedValueOnce(member('member-2', 2, 'TENANT_MEMBER_STATUS_SUSPENDED'))
      .mockResolvedValueOnce(member('member-3', 1, 'TENANT_MEMBER_STATUS_ACTIVE'))
    const result = await store.inspectMemberStatusBatch(targets, 'suspend')
    expect(result.map((item) => [item.id, item.state])).toEqual([
      ['member-2', 'confirmed'],
      ['member-3', 'not_applied'],
    ])
    expect(remote.get.mock.calls.map((call) => call[1])).toEqual(['member-2', 'member-3'])
    expect(remote.suspend).not.toHaveBeenCalled()
    expect(remote.activate).not.toHaveBeenCalled()
  })
  it('keeps failed inspection reads unknown while continuing to inspect the remaining targets', async () => {
    const store = useEnterpriseStore()
    remote.get
      .mockRejectedValueOnce(new Error('inspection read failed'))
      .mockResolvedValueOnce(member('member-3', 1, 'TENANT_MEMBER_STATUS_ACTIVE'))
    const result = await store.inspectMemberStatusBatch(targets, 'suspend')
    expect(result[0]?.state).toBe('unknown')
    expect(result[0]?.message).toContain('inspection read failed')
    expect(result[1]?.state).toBe('not_applied')
    expect(remote.get).toHaveBeenCalledTimes(2)
    expect(remote.suspend).not.toHaveBeenCalled()
  })
})
