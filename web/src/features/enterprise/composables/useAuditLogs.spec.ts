import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { useAuditLogs } from './useAuditLogs'

const mocks = vi.hoisted(() => ({
  store: { sourceKind: 'api', logs: [] as Array<Record<string, unknown>>, audit: vi.fn() },
  toast: vi.fn(),
  downloadCsv: vi.fn(),
  session: vi.fn(),
  list: vi.fn(),
  detail: vi.fn(),
  export: vi.fn(),
  switchTenant: vi.fn(),
  logout: vi.fn(),
  requestId: vi.fn(),
}))

vi.mock('@/stores/enterprise', () => ({ useEnterpriseStore: () => mocks.store }))
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ toast: mocks.toast }) }))
vi.mock('@/utils/format', () => ({ downloadCsv: mocks.downloadCsv }))
vi.mock('@/services/runtime/api', () => ({ loginUrl: () => '/login', logoutSession: mocks.logout }))
vi.mock('@/services/enterprise/auditRuntime', () => ({
  auditRequestId: mocks.requestId,
  auditRuntimeError: (error: unknown) => error instanceof Error ? error.message : 'request failed',
  readEnterpriseAuditSession: mocks.session,
  listEnterpriseAuditRecords: mocks.list,
  getEnterpriseAuditRecord: mocks.detail,
  exportEnterpriseAuditRecords: mocks.export,
  switchEnterpriseAuditTenant: mocks.switchTenant,
  sameAuditSession: (expected: { active_tenant_id?: string }, current: { active_tenant_id?: string }) =>
    expected.active_tenant_id === current.active_tenant_id,
}))

const session = { authenticated: true, active_tenant_id: 'tenant-a', user_id: 'user-a' }
const record = {
  auditId: 'audit-a', occurredAt: '2026-09-15T10:00:00Z', actorSubject: 'user:user-a',
  actorUserId: 'user-a', authMethod: 'session', authChannel: 'web-session',
  sessionRef: 'session-ref', requestId: 'request-a', idempotencyRef: 'idempotency-ref',
  operationId: 'tenant.member.suspend', module: 'access', target: 'user-b',
  result: 'success' as const, risk: 'high' as const, receiptRef: 'receipt-a',
  reason: 'member left', requestDigest: 'request-digest',
}
const pageResult = { records: [record], total: 1, page: 1, pageSize: 20 }
let wrapper: VueWrapper | undefined

async function start() {
  let model!: ReturnType<typeof useAuditLogs>
  wrapper = mount(defineComponent({
    setup() {
      model = useAuditLogs()
      return () => h('div')
    },
  }))
  await flushPromises()
  return model
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.store.sourceKind = 'api'
  mocks.store.logs = []
  mocks.session.mockReset().mockResolvedValue(session)
  mocks.list.mockReset().mockResolvedValue(pageResult)
  mocks.detail.mockReset().mockResolvedValue(record)
  mocks.export.mockReset().mockResolvedValue({ exportId: 'export-a', records: [record] })
  mocks.requestId.mockReset().mockReturnValue('enterprise-audit-export-a')
  mocks.switchTenant.mockReset().mockResolvedValue(undefined)
  mocks.logout.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

describe('audit page state', () => {
  it('keeps preview initialization separate from trusted server reads', async () => {
    mocks.store.sourceKind = 'demo'
    const model = await start()
    expect(model.apiMode.value).toBe(false)
    expect(mocks.session).not.toHaveBeenCalled()
    expect(mocks.list).not.toHaveBeenCalled()
  })

  it('surfaces server read failure without copying preview rows or reporting success', async () => {
    mocks.store.logs = [{ id: 'preview-only', action: 'preview action', risk: 'low' }]
    mocks.list.mockRejectedValue(new Error('audit read denied'))
    const model = await start()
    expect(model.serverError.value).toBe('audit read denied')
    expect(model.serverRecords.value).toEqual([])
    expect(model.serverTotal.value).toBe(0)
    expect(model.serverNotice.value).toBe('')
    expect(model.serverBusy.value).toBe(false)
    expect(mocks.store.audit).not.toHaveBeenCalled()
  })

  it('reuses the export key after failure and downloads only a server-confirmed result', async () => {
    const model = await start()
    mocks.export.mockRejectedValueOnce(new Error('export conflict'))
    await model.exportServerLogs()
    expect(model.serverNotice.value).toBe('')
    expect(model.exportRetry.value?.key).toBe('enterprise-audit-export-a')
    expect(mocks.downloadCsv).not.toHaveBeenCalled()
    await model.exportServerLogs()
    expect(mocks.export).toHaveBeenNthCalledWith(1, session, {}, 'enterprise-audit-export-a')
    expect(mocks.export).toHaveBeenNthCalledWith(2, session, {}, 'enterprise-audit-export-a')
    expect(mocks.requestId).toHaveBeenCalledTimes(1)
    expect(mocks.downloadCsv).toHaveBeenCalledTimes(1)
    expect(model.serverNotice.value).toContain('服务端导出已完成：1 条')
    expect(model.exportRetry.value).toBeNull()
    expect(mocks.store.audit).not.toHaveBeenCalled()
  })

  it('discards a pending server read when the page is unmounted', async () => {
    let resolve!: (value: typeof pageResult) => void
    mocks.list.mockReturnValue(new Promise<typeof pageResult>((done) => { resolve = done }))
    const model = await start()
    expect(model.serverBusy.value).toBe(true)
    wrapper?.unmount()
    wrapper = undefined
    resolve(pageResult)
    await flushPromises()
    expect(model.serverRecords.value).toEqual([])
    expect(model.serverNotice.value).toBe('')
  })
})
