import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  CommercialApiError,
  confirmSubscriptionChange,
  createEntitlementOverride,
  createPlanDraft,
  explainTenantEntitlements,
  listEntitlementOverrides,
  listPlanVersions,
  listPlatformModules,
  previewSubscriptionChange,
  revokeEntitlementOverride,
  type CreatePlanDraftInput,
} from './platformCommercial'

describe('CE-13 platform commercial service', () => {
  afterEach(() => vi.restoreAllMocks())

  it('uses the real module endpoint with server session cookies and no browser API key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ modules: [{ moduleCode: 'customer', name: '客户管理' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const modules = await listPlatformModules()

    expect(modules[0]?.moduleCode).toBe('customer')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0] ?? []
    expect(String(url)).toContain('/v1/platform/modules')
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).has('Authorization')).toBe(false)
  })

  it('keeps a forbidden trusted session distinguishable from an empty response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 403 }))

    await expect(listPlatformModules()).rejects.toMatchObject<Partial<CommercialApiError>>({
      status: 403,
      code: 'forbidden',
    })
  })

  it('lists versions only for an explicit plan code and encodes pagination using the generated contract names', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ versions: [{ planCode: 'office/pro', version: '2' }], nextAfterVersion: '2' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const result = await listPlanVersions('office/pro', { afterVersion: 1, pageSize: 20 })

    expect(result.versions[0]?.planCode).toBe('office/pro')
    expect(result.nextAfterVersion).toBe('2')
    const [url, init] = fetchMock.mock.calls[0] ?? []
    expect(String(url)).toContain('/v1/platform/plans/office%2Fpro/versions?afterVersion=1&pageSize=20')
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).has('Authorization')).toBe(false)
  })

  it('obtains CSRF from the trusted session before an unsafe plan mutation and never synthesizes an API key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-123' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ planCode: 'office-pro', version: '1', state: 'DRAFT' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    const input: CreatePlanDraftInput = {
      requestId: 'req-1',
      planCode: 'office-pro',
      name: '办公室专业版',
      reason: 'CE-13 service contract test',
      terms: {
        modules: [],
        salesScope: ['default'],
        validityMode: 'unlimited',
        validityDays: 0,
        priceRef: 'price:office-pro',
      },
    }

    await createPlanDraft(input)

    expect(fetchMock).toHaveBeenCalledTimes(2)
    const [sessionUrl, sessionInit] = fetchMock.mock.calls[0] ?? []
    expect(String(sessionUrl)).toContain('/auth/session')
    expect(sessionInit?.credentials).toBe('include')
    expect(new Headers(sessionInit?.headers).has('Authorization')).toBe(false)

    const [planUrl, planInit] = fetchMock.mock.calls[1] ?? []
    const headers = new Headers(planInit?.headers)
    expect(String(planUrl)).toContain('/v1/platform/plans')
    expect(planInit?.method).toBe('POST')
    expect(planInit?.credentials).toBe('include')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-123')
    expect(headers.get('Idempotency-Key')).toBe('req-1')
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(planInit?.body))).toMatchObject({ planCode: 'office-pro', requestId: 'req-1' })
  })

  it('fails closed before a mutation when the trusted session has no CSRF authority', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ authenticated: false }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await expect(createPlanDraft({
      requestId: 'req-no-session',
      planCode: 'blocked',
      name: 'Blocked',
      reason: 'test',
      terms: { modules: [], salesScope: ['default'], validityMode: 'unlimited', validityDays: 0, priceRef: '' },
    })).rejects.toMatchObject<Partial<CommercialApiError>>({ code: 'unauthenticated' })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('surfaces CAS conflicts distinctly so the UI can reread instead of overwriting', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: 'revision conflict' }), { status: 409 }))

    await expect(createPlanDraft({
      requestId: 'req-conflict',
      planCode: 'conflict',
      name: 'Conflict',
      reason: 'test',
      terms: { modules: [], salesScope: ['default'], validityMode: 'unlimited', validityDays: 0, priceRef: '' },
    })).rejects.toMatchObject<Partial<CommercialApiError>>({ status: 409, code: 'conflict' })
  })

  it('reads tenant override sources only for an explicit tenant id and never adds browser authority headers', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ sources: [{ id: 'ov-1', tenantId: 'tenant/a' }], sourceVersion: '7' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const result = await listEntitlementOverrides('tenant/a')

    expect(result.sourceVersion).toBe('7')
    expect(result.sources[0]?.id).toBe('ov-1')
    const [url, init] = fetchMock.mock.calls[0] ?? []
    expect(String(url)).toContain('/v1/platform/tenants/tenant%2Fa/entitlement-overrides')
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).has('Authorization')).toBe(false)
  })

  it('explains entitlements through trusted-session CSRF and preserves explicit capability filters', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-ent' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ tenantId: 'tenant-1', decisions: [] }), { status: 200 }))

    await explainTenantEntitlements('tenant-1', ['device.lifecycle', ' customer.view '])

    const [url, init] = fetchMock.mock.calls[1] ?? []
    const headers = new Headers(init?.headers)
    expect(String(url)).toContain('/v1/platform/tenants/tenant-1/entitlements')
    expect(init?.method).toBe('POST')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-ent')
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(init?.body))).toMatchObject({
      tenantId: 'tenant-1',
      capabilityCodes: ['device.lifecycle', 'customer.view'],
    })
  })

  it('uses the aggregate source version for create and revoke CAS without exposing an API key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-create' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ sourceVersion: '8', source: { id: 'ov-2' } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-revoke' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ sourceVersion: '9', source: { id: 'ov-2', revokedAt: 'now' } }), { status: 200 }))

    await createEntitlementOverride('tenant-1', {
      requestId: 'create-override',
      expectedVersion: '7',
      moduleCode: 'device',
      target: 'ENTITLEMENT_TARGET_CAPABILITY',
      key: 'device.lifecycle',
      fieldAction: '',
      effect: 'ENTITLEMENT_EFFECT_GRANT',
      effectiveAt: '',
      expiresAt: '',
      reason: 'temporary grant',
    })
    await revokeEntitlementOverride('tenant-1', 'ov-2', {
      requestId: 'revoke-override',
      expectedVersion: '8',
      reason: 'grant no longer required',
    })

    const createInit = fetchMock.mock.calls[1]?.[1]
    const revokeInit = fetchMock.mock.calls[3]?.[1]
    expect(JSON.parse(String(createInit?.body))).toMatchObject({ tenantId: 'tenant-1', expectedVersion: '7' })
    expect(JSON.parse(String(revokeInit?.body))).toMatchObject({ tenantId: 'tenant-1', id: 'ov-2', expectedVersion: '8' })
    expect(new Headers(createInit?.headers).has('Authorization')).toBe(false)
    expect(new Headers(revokeInit?.headers).has('Authorization')).toBe(false)
  })

  it('binds preview request_id to transport Idempotency-Key and trusted-session CSRF', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-preview' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ changeId: 'chg-1', previewHash: 'hash-1' }), { status: 200 }))

    await previewSubscriptionChange('tenant/a', {
      requestId: 'preview-key-1',
      action: 'SWITCH',
      targetPlanCode: 'office-pro',
      targetPlanVersion: '2',
      effectiveAt: '',
      reason: '平台人工切换套餐',
    })

    const [url, init] = fetchMock.mock.calls[1] ?? []
    const headers = new Headers(init?.headers)
    expect(String(url)).toContain('/v1/platform/tenants/tenant%2Fa/subscription/change-previews')
    expect(init?.credentials).toBe('include')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-preview')
    expect(headers.get('Idempotency-Key')).toBe('preview-key-1')
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(init?.body))).toMatchObject({
      tenantId: 'tenant/a',
      requestId: 'preview-key-1',
      action: 'SWITCH',
      targetPlanCode: 'office-pro',
      targetPlanVersion: '2',
    })
  })

  it('confirms exactly the preview hash with a distinct matching transport idempotency key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-confirm' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ changeId: 'chg-1', status: 'APPLIED' }), { status: 200 }))

    await confirmSubscriptionChange('tenant-1', 'chg-1', {
      requestId: 'confirm-key-1',
      previewHash: 'preview-sha256',
      reason: '已核对降级影响并人工批准',
    })

    const [url, init] = fetchMock.mock.calls[1] ?? []
    const headers = new Headers(init?.headers)
    expect(String(url)).toContain('/v1/platform/tenants/tenant-1/subscription/changes/chg-1/confirm')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-confirm')
    expect(headers.get('Idempotency-Key')).toBe('confirm-key-1')
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(init?.body))).toMatchObject({
      tenantId: 'tenant-1',
      changeId: 'chg-1',
      requestId: 'confirm-key-1',
      previewHash: 'preview-sha256',
    })
  })
})
