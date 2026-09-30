import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPlatformCommercialFeature, transitionPlatformCommercialFeature } from './platformFeatures'
import type { CommercialFeatureDTO } from './platformCommercial'

const feature: CommercialFeatureDTO = { featureCode: 'marketing-campaigns', name: '营销活动', version: '7', moduleRefs: [{ moduleCode: 'access-management', capabilityCodes: ['tenant.lifecycle'] }], productState: 'DRAFT', salesState: 'STOP_SELL', runtimeState: 'CONTINUING', migrationState: 'NONE', replacementCode: '' }

afterEach(() => vi.restoreAllMocks())

describe('platformFeatures', () => {
  it('uses trusted-session CSRF, idempotency and authoritative version for create and lifecycle writes', async () => {
    const calls: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      calls.push({ url: String(input), init })
      if (String(input).endsWith('/auth/session')) return new Response(JSON.stringify({ authenticated: true, csrf_token: 'csrf-feature' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      return new Response(JSON.stringify({ ...feature, productState: 'PUBLISHED', salesState: 'SELLABLE', version: '8' }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }))

    await createPlatformCommercialFeature({ featureCode: feature.featureCode, name: feature.name, moduleRefs: feature.moduleRefs, reason: '创建功能' })
    await transitionPlatformCommercialFeature(feature, 'publish', '发布功能')

    expect(calls).toHaveLength(4)
    for (const call of [calls[1]!, calls[3]!]) {
      const headers = new Headers(call.init?.headers)
      expect(headers.get('X-CSRF-Token')).toBe('csrf-feature')
      expect(headers.get('Idempotency-Key')).toMatch(/^commercial-feature-/)
      expect(headers.has('Authorization')).toBe(false)
    }
    expect(calls[1]?.url).toContain('/v1/platform/commercial-features')
    expect(JSON.parse(String(calls[3]?.init?.body))).toMatchObject({ featureCode: feature.featureCode, version: '7', reason: '发布功能' })
  })
})
