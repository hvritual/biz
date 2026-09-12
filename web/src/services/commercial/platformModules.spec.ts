import { afterEach, describe, expect, it, vi } from 'vitest'
import { setPlatformModuleSalesStatus } from './platformModules'
import type { ModuleDTO } from './platformCommercial'

const module: ModuleDTO = {
  moduleCode: 'customer',
  name: '客户管理',
  category: 'business',
  salesScope: ['default'],
  technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
  salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
  capabilityCodes: ['customer.read'],
  quotaSchemaKeys: [],
  fieldPolicySchemaKeys: [],
  dependencies: [],
  version: '7',
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('platformModules', () => {
  it('uses trusted session CSRF and idempotency without browser API keys', async () => {
    const calls: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, init })
      if (url.endsWith('/auth/session')) {
        return new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-module' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({ ...module, salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: '8' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }))

    const result = await setPlatformModuleSalesStatus(module, 'MODULE_SALES_STATUS_RETIRED', '停售验证')
    expect(result.version).toBe('8')
    expect(calls).toHaveLength(2)
    const mutation = calls[1]!
    const headers = new Headers(mutation.init?.headers)
    expect(headers.get('X-CSRF-Token')).toBe('csrf-module')
    expect(headers.get('Idempotency-Key')).toMatch(/^ce13-module-sales-/)
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(mutation.init?.body))).toMatchObject({
      moduleCode: 'customer',
      salesStatus: 'MODULE_SALES_STATUS_RETIRED',
      version: '7',
      reason: '停售验证',
    })
  })
})
