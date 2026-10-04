import { describe, expect, it } from 'vitest'
import { knownQuotaNumber, planChangeAccess } from './planAccess'

const operation = 'commercial.subscription.change.targets_my'
const session = { authenticated: true, actor_kind: 'tenant', user_id: 'u1', active_tenant_id: 't1', context_version: 1 }
const projection = () => ({
  authenticated: true, user_id: 'u1', tenant_id: 't1', button_codes: [operation],
  grants: [{ permission: 'tenant.subscription.manage' }, { permission: 'commercial.catalog.read' }],
  modules: [{ allowed: true, actions: [operation] }],
})

describe('plan access feedback consumes the current authorization projection', () => {
  it('keeps unresolved authorization distinct from a denial', () => {
    expect(planChangeAccess(null, session, 'loading', true)).toBe('checking')
    expect(planChangeAccess(projection(), session, 'ready', false)).toBe('checking')
    expect(planChangeAccess(null, session, 'error', true)).toBe('unknown')
    expect(planChangeAccess(null, session, 'unauthenticated', true)).toBe('login')
  })
  it('uses the granted operation instead of recreating permission grants', () => {
    const snapshot = { ...projection(), grants: [] }
    expect(planChangeAccess(snapshot, session, 'ready', true)).toBe('allowed')
  })
  it('identifies missing member permission without offering a purchase workaround', () => {
    const snapshot = { ...projection(), button_codes: [], grants: [] }
    expect(planChangeAccess(snapshot, session, 'ready', true)).toBe('iam')
  })
  it('separates related unavailable plan capability from member permissions', () => {
    const snapshot = { ...projection(), modules: [{ allowed: false, actions: [operation] }] }
    expect(planChangeAccess(snapshot, session, 'ready', true)).toBe('entitlement')
  })
  it('does not use an unrelated module restriction as a plan-management denial', () => {
    const snapshot = { ...projection(), modules: [{ allowed: false, actions: ['device.update'] }] }
    expect(planChangeAccess(snapshot, session, 'ready', true)).toBe('allowed')
  })
  it('does not infer access when permissions exist but the operation is absent', () => {
    expect(planChangeAccess({ ...projection(), button_codes: [] }, session, 'ready', true)).toBe('unknown')
  })
  it('rejects different tenant and principal projections', () => {
    expect(planChangeAccess({ ...projection(), tenant_id: 't2' }, session, 'ready', true)).toBe('unknown')
    expect(planChangeAccess({ ...projection(), user_id: 'u2' }, session, 'ready', true)).toBe('unknown')
  })
})

describe('unknown quota values are never presented as zero', () => {
  it.each([undefined, null, '', '  ', false, true, NaN, Infinity, -1, 'invalid'])('keeps %s unknown', (value) => {
    expect(knownQuotaNumber(value)).toBeNull()
  })
  it('preserves real zero and known usage', () => {
    expect(knownQuotaNumber(0)).toBe(0)
    expect(knownQuotaNumber('0')).toBe(0)
    expect(knownQuotaNumber('18')).toBe(18)
  })
})
