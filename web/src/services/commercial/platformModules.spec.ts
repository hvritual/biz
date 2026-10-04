import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPlatformModule, listPlatformModuleDefinitions, setPlatformModuleSalesStatus } from './platformModules'
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
  it('reads code-owned module definitions from the existing trusted action catalog', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
      expect(String(input)).toContain('/api/auth/action-catalog')
      return new Response(JSON.stringify({
        module_definitions: [{
          moduleCode: 'device-operations', capabilityCodes: ['device.lifecycle'], quotaSchemaKeys: ['tenant.devices'],
          fieldPolicySchemaKeys: ['device.identity'], dependencies: [], implementationReady: true,
        }],
      }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }))
    await expect(listPlatformModuleDefinitions()).resolves.toEqual([
      expect.objectContaining({ moduleCode: 'device-operations', implementationReady: true }),
    ])
  })

  it('creates a registered module through trusted session CSRF and idempotency', async () => {
    const calls: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, init })
      if (url.endsWith('/auth/session')) {
        return new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-module' }), {
          status: 200, headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify({
        ...module, moduleCode: 'device-operations', name: '设备运营', category: 'device',
        salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: '1',
      }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }))

    const result = await createPlatformModule({
      moduleCode: 'device-operations', name: '设备运营', category: 'device', salesScope: ['default'], reason: '纳入平台目录',
    })
    expect(result.version).toBe('1')
    expect(calls).toHaveLength(2)
    const mutation = calls[1]!
    const headers = new Headers(mutation.init?.headers)
    expect(mutation.url).toContain('/api/v1/platform/modules')
    expect(mutation.init?.method).toBe('POST')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-module')
    expect(headers.get('Idempotency-Key')).toMatch(/^platform-module-create-/)
    expect(headers.has('Authorization')).toBe(false)
    expect(JSON.parse(String(mutation.init?.body))).toMatchObject({
      moduleCode: 'device-operations', name: '设备运营', category: 'device', salesScope: ['default'], reason: '纳入平台目录',
    })
  })

  it('keeps sales-status updates on the same catalog version', async () => {
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
      return new Response(JSON.stringify({ ...module, salesStatus: 'MODULE_SALES_STATUS_RETIRED', version: '7' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }))

    const result = await setPlatformModuleSalesStatus(module, 'MODULE_SALES_STATUS_RETIRED', '停售验证')
    expect(result.version).toBe('7')
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
