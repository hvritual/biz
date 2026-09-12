import { describe, expect, it } from 'vitest'
import { routeContentEnabled } from './dataMode'

describe('routeContentEnabled', () => {
  it('keeps preview pages available in demo mode', () => {
    expect(routeContentEnabled(true, undefined)).toBe(true)
  })

  it('allows explicitly platform-backed routes in API mode', () => {
    expect(routeContentEnabled(false, 'platform')).toBe(true)
  })

  it('fails closed for preview-only routes in API mode', () => {
    expect(routeContentEnabled(false, 'tenant')).toBe(false)
    expect(routeContentEnabled(false, undefined)).toBe(false)
  })
})
