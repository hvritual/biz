import { describe, expect, it } from 'vitest'
import { routeContentEnabled } from './dataMode'

describe('routeContentEnabled', () => {
  it('keeps preview pages available in demo mode', () => {
    expect(routeContentEnabled(true, undefined)).toBe(true)
  })

  it('keeps explicitly platform-backed routes available in API mode', () => {
    expect(routeContentEnabled(false, 'platform')).toBe(true)
  })

  it('keeps pages without an API adapter visible as complete demo surfaces in API mode', () => {
    expect(routeContentEnabled(false, 'tenant')).toBe(true)
    expect(routeContentEnabled(false, undefined)).toBe(true)
  })
})
