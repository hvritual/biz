import { describe, expect, it } from 'vitest'
import { initialSubscriptionReadbackMatches, type InitialSubscriptionReadback } from './initialSubscriptionReadback'
import type { EntitlementDecisionDTO, PlanVersionDTO, TenantSubscriptionDTO } from './platformCommercial'

function fixture(): InitialSubscriptionReadback {
  const target: PlanVersionDTO = {
    planCode: 'office-pro', version: '3', revision: '1', planRevision: '3', state: 'PUBLISHED', name: 'Office Pro',
    terms: { modules: [{ moduleCode: 'device', capabilityCodes: ['device.lifecycle'],
      quotas: [{ key: 'device.count', unlimited: false, value: '100' }], fields: [] }],
    salesScope: ['office'], validityMode: 'fixed_days', validityDays: 365, priceRef: '' },
    contentSha256: 'a'.repeat(64), createdAt: '', publishedAt: '', retiredAt: '', actorId: 'admin', reason: 'publish',
  }
  const subscription: TenantSubscriptionDTO = {
    subscriptionId: 'sub-first', tenantId: 'tenant-a', kind: 'BASE', state: 'ACTIVE', planCode: 'office-pro', planVersion: '3',
    ruleId: '', ruleVersion: '0', salesScope: 'office', entitlementSourceVersion: '7', createdAt: '', matchExplanation: '',
    revision: '1', periodStart: '', periodEnd: '', renewalStopped: false, pendingChangeId: '',
  }
  const decision = (kind: string, key: string): EntitlementDecisionDTO => ({
    kind, key, moduleCode: 'device', fieldAction: '', allowed: true, reason: 'ALLOWED', masked: false, sources: [],
  })
  return {
    tenantId: 'tenant-a', salesScope: 'office', target, subscription,
    preview: {
      changeId: 'chg-first', tenantId: 'tenant-a', actorId: 'admin', requestId: 'preview-first', action: 'INITIAL',
      classification: 'INITIAL_ACTIVATION', mode: 'IMMEDIATE', previewHash: 'b'.repeat(64), target,
      subscriptionRevision: '0', sourceVersion: '0', entitlementVersion: '1', catalogRevision: '3',
      createdAt: '', expiresAt: '', effectiveAt: '', entitlementExpiresAt: '', dependencies: [], quotaImpacts: [],
      impacts: [], impactDetails: [], pricingBasis: '', quotaValidationRequired: false, provisioningRequirements: [],
    },
    receipt: {
      changeId: 'chg-first', tenantId: 'tenant-a', actorId: 'admin', requestId: 'confirm-first', previewHash: 'b'.repeat(64),
      action: 'INITIAL', status: 'APPLIED', mode: 'IMMEDIATE', confirmedAt: '', effectiveAt: '', entitlementExpiresAt: '',
      reason: 'confirm', after: { ...subscription }, beforeSourceVersion: '0', afterSourceVersion: '7',
      beforeEntitlementVersion: '1', afterEntitlementVersion: '11', quotaValidationRequired: false, pricingAuthority: '',
      quotaImpacts: [], provisioningTaskId: '', failureCode: '',
    },
    entitlements: {
      tenantId: 'tenant-a', sourceVersion: '7', resolverVersion: '4', evaluatedAt: '', validUntil: '', nextTransitionAt: '',
      catalogVersions: [{ moduleCode: 'device', version: '3' }], entitlementVersion: '11', catalogRevision: '3',
      permissionVersion: '', permissionSubject: '', decisions: [decision('module', 'device'),
        decision('capability', 'device.lifecycle'),
        { ...decision('quota', 'device.count'), limit: { unlimited: false, value: '100' } }],
    },
  }
}

function getDecision(input: InitialSubscriptionReadback, kind: string) {
  const result = input.entitlements.decisions.find((item) => item.kind === kind)
  if (!result) throw new Error(`Missing fixture decision ${kind}`)
  return result
}

function getModule(input: InitialSubscriptionReadback) {
  const result = input.target.terms.modules[0]
  if (!result) throw new Error('Missing fixture module')
  return result
}

const refusalCases: [string, (input: InitialSubscriptionReadback) => void][] = [
  ['denied capability', (i) => { getDecision(i, 'capability').allowed = false }],
  ['denied module', (i) => { getDecision(i, 'module').allowed = false }],
  ['missing module decision', (i) => { i.entitlements.decisions = i.entitlements.decisions.filter((d) => d.kind !== 'module') }],
  ['missing capability decision', (i) => { i.entitlements.decisions = i.entitlements.decisions.filter((d) => d.kind !== 'capability') }],
  ['duplicate capability decisions', (i) => { i.entitlements.decisions.push({ ...getDecision(i, 'capability'), allowed: false }) }],
  ['denied quota', (i) => { getDecision(i, 'quota').allowed = false }],
  ['quota without a limit', (i) => { delete getDecision(i, 'quota').limit }],
  ['quota below target', (i) => { getDecision(i, 'quota').limit = { unlimited: false, value: '99' } }],
  ['unsafe numeric quota', (i) => { getDecision(i, 'quota').limit = { unlimited: false, value: 9007199254740992 } }],
  ['restricted subscription', (i) => { i.subscription.state = 'RESTRICTED' }],
  ['ended subscription', (i) => { i.subscription.state = 'ENDED' }],
  ['provisioning subscription', (i) => { i.subscription.state = 'PROVISIONING' }],
  ['pending change', (i) => { i.subscription.pendingChangeId = 'other-change' }],
  ['wrong subscription tenant', (i) => { i.subscription.tenantId = 'tenant-b' }],
  ['wrong entitlement tenant', (i) => { i.entitlements.tenantId = 'tenant-b' }],
  ['wrong receipt tenant', (i) => { i.receipt.tenantId = 'tenant-b' }],
  ['wrong preview tenant', (i) => { i.preview.tenantId = 'tenant-b' }],
  ['wrong scope', (i) => { i.subscription.salesScope = 'rental' }],
  ['wildcard scope', (i) => { i.salesScope = '*' }],
  ['different subscription', (i) => { i.subscription.subscriptionId = 'other-subscription' }],
  ['different target version', (i) => { i.subscription.planVersion = '4' }],
  ['unpublished target', (i) => { i.target.state = 'DRAFT' }],
  ['wrong change receipt', (i) => { i.receipt.changeId = 'other-change' }],
  ['wrong preview hash', (i) => { i.receipt.previewHash = 'c'.repeat(64) }],
  ['non-initial receipt', (i) => { i.receipt.action = 'SWITCH' }],
  ['not-applied receipt', (i) => { i.receipt.status = 'PROVISIONING' }],
  ['failed receipt', (i) => { i.receipt.failureCode = 'PROVISIONING_FAILED' }],
  ['missing authoritative after', (i) => { delete i.receipt.after }],
  ['stale source counter', (i) => { i.entitlements.sourceVersion = '6' }],
  ['stale entitlement counter', (i) => { i.entitlements.entitlementVersion = '10' }],
  ['zero proof counter', (i) => { i.receipt.afterEntitlementVersion = '0' }],
  ['malformed proof counter', (i) => { i.receipt.afterEntitlementVersion = 'unknown'; i.entitlements.entitlementVersion = 'unknown' }],
  ['exponential counter', (i) => { i.entitlements.entitlementVersion = '1e2' }],
  ['uint64 overflow', (i) => { i.entitlements.entitlementVersion = '18446744073709551616' }],
  ['rounded numeric counter', (i) => { i.entitlements.entitlementVersion = 9007199254740992 }],
  ['empty target terms', (i) => { i.target.terms.modules = [] }],
]

describe('initial subscription authoritative readback', () => {
  it('confirms matching active subscription, receipt, and allowed target decisions', () => {
    expect(initialSubscriptionReadbackMatches(fixture())).toBe(true)
  })

  it.each(refusalCases)('does not confirm %s', (_name, mutate) => {
    const input = fixture()
    mutate(input)
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
  })

  it('accepts omitted protobuf false/zero defaults but not a missing grant or limit message', () => {
    const input = fixture()
    for (const decision of input.entitlements.decisions) delete (decision as Partial<EntitlementDecisionDTO>).masked
    const limit = getDecision(input, 'quota').limit
    if (!limit) throw new Error('Missing limit')
    delete (limit as Partial<typeof limit>).unlimited
    expect(initialSubscriptionReadbackMatches(input)).toBe(true)
    delete (getDecision(input, 'capability') as Partial<EntitlementDecisionDTO>).allowed
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
  })

  it('preserves adjacent uint64 counter precision above the safe integer boundary', () => {
    const input = fixture()
    input.receipt.afterEntitlementVersion = '9007199254740993'
    input.entitlements.entitlementVersion = '9007199254740992'
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
    input.entitlements.entitlementVersion = '9007199254740993'
    expect(initialSubscriptionReadbackMatches(input)).toBe(true)
  })

  it('accepts newer authoritative snapshots and additional quota without demanding equality', () => {
    const input = fixture()
    input.entitlements.sourceVersion = '8'
    input.entitlements.entitlementVersion = '12'
    getDecision(input, 'quota').limit = { unlimited: false, value: '150' }
    expect(initialSubscriptionReadbackMatches(input)).toBe(true)
  })

  it('does not substitute a finite quota for an unlimited target', () => {
    const input = fixture()
    const quota = getModule(input).quotas[0]
    if (!quota) throw new Error('Missing quota')
    quota.unlimited = true
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
    getDecision(input, 'quota').limit = { unlimited: true, value: '0' }
    expect(initialSubscriptionReadbackMatches(input)).toBe(true)
  })

  it.each(['allow', 'masked', 'deny'])('checks field action and %s mode rather than key presence', (mode) => {
    const input = fixture()
    getModule(input).fields = [{ key: 'device.identity', action: 'read', mode }]
    const field: EntitlementDecisionDTO = {
      kind: 'field', moduleCode: 'device', key: 'device.identity', fieldAction: 'read',
      allowed: mode !== 'deny', masked: mode === 'masked', reason: '', sources: [],
    }
    input.entitlements.decisions.push(field)
    expect(initialSubscriptionReadbackMatches(input)).toBe(true)
    field.allowed = !field.allowed
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
    field.allowed = !field.allowed
    field.fieldAction = 'export'
    expect(initialSubscriptionReadbackMatches(input)).toBe(false)
  })
})
