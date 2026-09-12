import { afterEach, describe, expect, it, vi } from 'vitest'
import { CommercialApiError, listPlatformModules } from './platformCommercial'

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
})
