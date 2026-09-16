import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { TrustedSession } from '@/services/runtime/api'

const mocks = vi.hoisted(() => ({
  request: vi.fn(),
  mutate: vi.fn(),
  readSession: vi.fn(),
  selectSessionTenant: vi.fn(),
  commercialRequestId: vi.fn(() => 'enterprise-audit-export-test'),
}))

vi.mock('@/services/commercial/platformCommercial', () => ({
  CommercialApiError: class CommercialApiError extends Error {
    constructor(
      message: string,
      readonly status: number,
      readonly code: string,
    ) {
      super(message)
    }
  },
  commercialRequestId: mocks.commercialRequestId,
  mutate: mocks.mutate,
  request: mocks.request,
}))

vi.mock('@/services/runtime/api', () => ({
  readSession: mocks.readSession,
  selectSessionTenant: mocks.selectSessionTenant,
  sessionContext: (session: TrustedSession) => JSON.stringify({
    actor_kind: session.actor_kind ?? '',
    platform_subject: session.platform_subject ?? '',
    user_id: session.user_id ?? '',
    active_tenant_id: session.active_tenant_id ?? '',
  }),
}))

import {
  auditRequestId,
  exportEnterpriseAuditRecords,
  listEnterpriseAuditRecords,
} from './auditRuntime'

const session: TrustedSession = {
  authenticated: true,
  actor_kind: 'tenant_user',
  user_id: 'user-1',
  active_tenant_id: 'tenant-1',
}

const record = {
  auditId: 'audit-1',
  occurredAt: '2026-09-15T10:00:00Z',
  actorSubject: 'user:user-1',
  actorUserId: 'user-1',
  authMethod: 'session',
  authChannel: 'web-session',
  sessionRef: 'sha256:session',
  requestId: 'req-1',
  idempotencyRef: 'sha256:idem',
  operationId: 'access.audit.export',
  module: 'access',
  target: 'tenant:tenant-1',
  result: 'success' as const,
  risk: 'high' as const,
  receiptRef: 'request:req-1',
  reason: 'qualification',
  requestDigest: 'digest-1',
}

describe('enterprise audit runtime', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists the trusted tenant audit read model with server-side filters and paging', async () => {
    mocks.request.mockResolvedValue({ records: [record], total: 31, page: 2, pageSize: 50 })

    const result = await listEnterpriseAuditRecords(session, {
      query: 'user-1',
      operationId: 'access.audit.export',
      result: 'success',
      risk: 'high',
      page: 2,
      pageSize: 50,
    })

    expect(mocks.request).toHaveBeenCalledTimes(1)
    const [path, options] = mocks.request.mock.calls[0]
    expect(path).toContain('/v1/tenant/audit-logs?')
    expect(path).toContain('query=user-1')
    expect(path).toContain('operation_id=access.audit.export')
    expect(path).toContain('result=success')
    expect(path).toContain('risk=high')
    expect(path).toContain('page=2')
    expect(path).toContain('page_size=50')
    expect(options.headers['X-Biz-Session-Context']).toContain('tenant-1')
    expect(result.total).toBe(31)
    expect(result.records[0]).toMatchObject({ auditId: 'audit-1', risk: 'high' })
    expect(Object.isFrozen(result.records[0])).toBe(true)
  })

  it('exports through the audited server mutation with a trusted session and idempotency key', async () => {
    mocks.mutate.mockResolvedValue({
      exportId: 'export-req-2',
      generatedAt: '2026-09-15T10:01:00Z',
      records: [record],
    })

    const result = await exportEnterpriseAuditRecords(
      session,
      { query: 'user-1', result: 'success', risk: 'high' },
      'idem-export-1',
      1000,
    )

    expect(mocks.mutate).toHaveBeenCalledWith(
      '/v1/tenant/audit-logs/exports',
      'POST',
      {
        query: 'user-1',
        operationId: '',
        result: 'success',
        risk: 'high',
        maxRows: 1000,
      },
      expect.objectContaining({ idempotencyKey: 'idem-export-1' }),
    )
    const options = mocks.mutate.mock.calls[0][3]
    expect(options.sessionContext).toContain('tenant-1')
    expect(result.exportId).toBe('export-req-2')
    expect(result.records).toHaveLength(1)
  })

  it('rejects audit reads without a trusted tenant context', async () => {
    await expect(listEnterpriseAuditRecords({ authenticated: true }, {})).rejects.toThrow('请选择可访问的租户')
    expect(mocks.request).not.toHaveBeenCalled()
  })

  it('creates an audit-specific export request id', () => {
    expect(auditRequestId()).toBe('enterprise-audit-export-test')
    expect(mocks.commercialRequestId).toHaveBeenCalledWith('enterprise-audit-export')
  })
})
