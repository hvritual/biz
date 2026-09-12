import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  CommercialApiError,
  createPlanDraft,
  listPlanVersions,
  listPlatformModules,
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
})
