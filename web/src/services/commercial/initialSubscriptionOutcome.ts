import type {
  EntitlementDecisionDTO,
  EntitlementView,
  PlanVersionDTO,
  SubscriptionChangePreviewDTO,
  SubscriptionChangeReceiptDTO,
  TenantSubscriptionDTO,
} from './platformCommercial'

export interface InitialSubscriptionReadback {
  tenantId: string
  salesScope: string
  target: PlanVersionDTO
  preview?: SubscriptionChangePreviewDTO | null
  receipt: SubscriptionChangeReceiptDTO
  subscription: TenantSubscriptionDTO
  entitlements: EntitlementView
}

// Protobuf uint64 counters may exceed Number.MAX_SAFE_INTEGER. A rounded number
// cannot prove freshness; require decimal strings (or still-safe integers).
function uint64(value: unknown): bigint | null {
  if (typeof value === 'number' && (!Number.isSafeInteger(value) || value < 0)) return null
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (!/^(0|[1-9]\d*)$/.test(String(value))) return null
  const parsed = BigInt(value)
  return parsed <= 18446744073709551615n ? parsed : null
}

function notOlder(actual: unknown, expected: unknown) {
  const current = uint64(actual)
  const minimum = uint64(expected)
  return current !== null && minimum !== null && minimum > 0n && current >= minimum
}

type DecisionKey = { kind: string; moduleCode: string; key: string; fieldAction: string }

function keysForTarget(target: PlanVersionDTO): DecisionKey[] {
  const keys: DecisionKey[] = []
  for (const module of target.terms.modules ?? []) {
    keys.push({ kind: 'module', moduleCode: module.moduleCode, key: module.moduleCode, fieldAction: '' })
    for (const capability of module.capabilityCodes ?? []) {
      keys.push({ kind: 'capability', moduleCode: module.moduleCode, key: capability, fieldAction: '' })
    }
    for (const quota of module.quotas ?? []) {
      keys.push({ kind: 'quota', moduleCode: module.moduleCode, key: quota.key, fieldAction: '' })
    }
    for (const field of module.fields ?? []) {
      keys.push({ kind: 'field', moduleCode: module.moduleCode, key: field.key, fieldAction: field.action })
    }
  }
  return keys
}

function uniqueDecision(view: EntitlementView, key: DecisionKey): EntitlementDecisionDTO | null {
  const found = view.decisions?.filter((item) => item.kind === key.kind
    && item.moduleCode === key.moduleCode && item.key === key.key
    && (item.fieldAction || '') === key.fieldAction) ?? []
  // Do not treat duplicated or conflicting rows as one authoritative decision.
  return found.length === 1 ? found[0]! : null
}

function validLimit(decision: EntitlementDecisionDTO, required: boolean): boolean {
  if (!decision.limit) return !required
  return decision.limit.unlimited === true || uint64(decision.limit.value ?? 0) !== null
}

function sameEffect(actual: EntitlementDecisionDTO, projected: EntitlementDecisionDTO): boolean {
  // Protojson may omit false booleans. Compare *effective* server policy, not
  // source IDs, timestamps, reason copy or raw plan grants.
  if ((actual.allowed === true) !== (projected.allowed === true)
    || (actual.masked === true) !== (projected.masked === true)) return false
  if (Boolean(actual.limit) !== Boolean(projected.limit)) return false
  if (!actual.limit || !projected.limit) return true
  if ((actual.limit.unlimited === true) !== (projected.limit.unlimited === true)) return false
  if (actual.limit.unlimited === true) return true
  const current = uint64(actual.limit.value ?? 0)
  const planned = uint64(projected.limit.value ?? 0)
  return current !== null && planned !== null && current === planned
}

function observedEffectiveDecisions(actual: EntitlementView, target: PlanVersionDTO, projected?: EntitlementView): boolean {
  if (!Array.isArray(target.terms?.modules) || !target.terms.modules.length
    || !Array.isArray(actual.decisions)) return false
  if (projected && (!Array.isArray(projected.decisions) || projected.tenantId !== actual.tenantId)) return false
  const keys = keysForTarget(target)
  for (const key of keys) {
    const current = uniqueDecision(actual, key)
    if (!current || !validLimit(current, key.kind === 'quota' && current.allowed === true)) return false
    if (projected) {
      const expected = uniqueDecision(projected, key)
      if (!expected || !validLimit(expected, key.kind === 'quota' && expected.allowed === true)
        || !sameEffect(current, expected)) return false
    }
  }
  return true
}

/** Confirms a readback, never grants access or recomputes server entitlements. */
export function initialSubscriptionReadbackMatches(input: InitialSubscriptionReadback): boolean {
  const { tenantId, salesScope, target, preview, receipt, subscription, entitlements } = input
  if (!tenantId || !salesScope || salesScope === '*' || !receipt.changeId
    || !/^[a-f0-9]{64}$/i.test(receipt.previewHash)) return false
  if (preview && (!preview.changeId || !preview.previewHash || preview.tenantId !== tenantId
    || preview.action !== 'INITIAL' || receipt.changeId !== preview.changeId
    || receipt.previewHash !== preview.previewHash)) return false
  if (receipt.tenantId !== tenantId || receipt.action !== 'INITIAL'
    || receipt.status !== 'APPLIED' || receipt.failureCode) return false
  if (target.state !== 'PUBLISHED' || !target.planCode || uint64(target.version) === null
    || uint64(target.version) === 0n) return false
  if (!receipt.after?.subscriptionId || subscription.subscriptionId !== receipt.after.subscriptionId
    || receipt.after.tenantId !== tenantId || subscription.tenantId !== tenantId
    || entitlements.tenantId !== tenantId || subscription.kind !== 'BASE'
    || (subscription.state !== 'ACTIVE' && subscription.state !== 'TRIAL') || subscription.pendingChangeId
    || subscription.salesScope !== salesScope || receipt.after.salesScope !== salesScope
    || (subscription.state === 'TRIAL' && target.terms.validityMode !== 'fixed_days')) return false
  if (subscription.planCode !== target.planCode || receipt.after.planCode !== target.planCode
    || uint64(subscription.planVersion) === null || uint64(receipt.after.planVersion) === null
    || String(subscription.planVersion) !== String(target.version)
    || String(receipt.after.planVersion) !== String(target.version)) return false
  if (!notOlder(subscription.revision, receipt.after.revision)
    || !notOlder(subscription.entitlementSourceVersion, receipt.afterSourceVersion)
    || !notOlder(entitlements.sourceVersion, subscription.entitlementSourceVersion)
    || !notOlder(entitlements.sourceVersion, receipt.afterSourceVersion)
    || !notOlder(entitlements.entitlementVersion, receipt.afterEntitlementVersion)) return false
  // Server projection includes existing override and safety authorities. A
  // denied/masked/reduced effective decision can be the *correct* final result.
  // Compare the projected effective policy when the actual source generation
  // matches it; a newer generation must instead reflect current server truth.
  const projected = preview?.projectedEntitlements
  if (projected && projected.tenantId !== tenantId) return false
  const expectedSource = projected ? uint64(projected.sourceVersion) : null
  if (projected && expectedSource === null) return false
  const actualSource = uint64(entitlements.sourceVersion)
  const sameGeneration = projected && expectedSource === actualSource
  return observedEffectiveDecisions(entitlements, target, sameGeneration ? projected : undefined)
}
