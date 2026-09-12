import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRuntimeApi, logoutSession } from './api'
function response(body: unknown, status = 200) {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
afterEach(() => vi.unstubAllGlobals())
describe('trusted runtime API', () => {
  it('keeps exact versions and the same idempotency key on a retry, without browser API keys', async () => {
    const requests: Array<{ url: string; init: RequestInit }> = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init: RequestInit) => {
        requests.push({ url, init })
        return url.endsWith('/auth/session')
          ? response({ authenticated: true, csrf_token: 'trusted-csrf' })
          : response({ id: 'device/1', version: '9007199254740994' })
      }),
    )
    const api = createRuntimeApi('one-user-command').tenantApi
    const device = {
      id: 'device/1',
      name: '旧名称',
      siteId: 'site-1',
      serial: 's',
      createdBy: 'u',
      version: '9007199254740993',
    }
    await api.transferDevice(device, 'site-2')
    await api.transferDevice(device, 'site-2')
    const writes = requests.filter((r) => r.init.method === 'PATCH')
    expect(writes).toHaveLength(2)
    for (const r of writes) {
      const h = new Headers(r.init.headers)
      expect(h.get('Authorization')).toBeNull()
      expect(h.get('X-CSRF-Token')).toBe('trusted-csrf')
      expect(h.get('Idempotency-Key')).toBe('one-user-command')
      expect(r.init.credentials).toBe('include')
      expect(r.url).toContain('device%2F1/transfer')
      expect(JSON.parse(String(r.init.body)).version).toBe(device.version)
    }
  })
  it('denies a mutation before sending it when the session is absent', async () => {
    const fetch = vi.fn(async () => response({ authenticated: false }))
    vi.stubGlobal('fetch', fetch)
    await expect(createRuntimeApi('k').tenantApi.inviteMember('member@example.com')).rejects.toMatchObject({
      status: 401,
    })
    expect(fetch).toHaveBeenCalledTimes(1)
  })
  it('preserves permission scope protocol values and reports server denial', async () => {
    const fetch = vi.fn(async (url: string) =>
      url.endsWith('/auth/session')
        ? response({ authenticated: true, csrf_token: 'c' })
        : response({ message: 'last owner protected' }, 403),
    )
    vi.stubGlobal('fetch', fetch)
    await expect(
      createRuntimeApi('k').tenantApi.setRolePermissions(
        { id: 'r', name: 'owner', status: 'active', version: '4' },
        [{ permission: 'device.read', scope: 'DATA_SCOPE_SITES' }],
      ),
    ).rejects.toMatchObject({ status: 403, message: 'last owner protected' })
  })
  it('accepts empty 204 logout responses', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) =>
        url.endsWith('/auth/session')
          ? response({ authenticated: true, csrf_token: 'c' })
          : response(null, 204),
      ),
    )
    await expect(logoutSession()).resolves.toBeUndefined()
  })
})
