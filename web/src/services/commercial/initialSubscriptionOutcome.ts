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

function allowed(decision: EntitlementDecisionDTO | undefined) {
  // protojson omits false scalar defaults; missing `allowed` must never grant.
  return decision?.allowed === true && (decision.masked === false || decision.masked === undefined)
}

function targetDecisionsConfirmed(view: EntitlementView, target: PlanVersionDTO) {
  const modules = target.terms?.modules
  if (!Array.isArray(modules) || !modules.length || !Array.isArray(view.decisions)) return false
  for (const module of modules) {
    const decision = (kind: string, key: string, action = '') => {
      const matches = view.decisions.filter((item) => item.moduleCode === module.moduleCode
        && item.kind === kind && item.key === key && (item.fieldAction || '') === action)
      // Conflicting/duplicate rows are not proof of a single effective decision.
      return matches.length === 1 ? matches[0] : undefined
    }
    if (!allowed(decision('module', module.moduleCode))) return false
    for (const capability of module.capabilityCodes ?? []) {
      if (!allowed(decision('capability', capability))) return false
    }
    for (const quota of module.quotas ?? []) {
      const actual = decision('quota', quota.key)
      if (!allowed(actual) || !actual?.limit) return false
      if (quota.unlimited) {
        if (actual.limit.unlimited !== true) return false
      } else {
        const expectedLimit = uint64(quota.value ?? 0)
        if (expectedLimit === null) return false
        if (actual.limit.unlimited === true) continue
        const actualLimit = uint64(actual.limit.value ?? 0)
        if ((actual.limit.unlimited !== false && actual.limit.unlimited !== undefined) || actualLimit === null || actualLimit < expectedLimit) return false
      }
    }
    for (const field of module.fields ?? []) {
      const actual = decision('field', field.key, field.action)
      if (!actual) return false
      if (field.mode === 'allow' && !allowed(actual)) return false
      if (field.mode === 'masked' && (actual.allowed !== true || actual.masked !== true)) return false
      if (field.mode === 'deny' && actual.allowed !== false && actual.allowed !== undefined) return false
      if (!['allow', 'masked', 'deny'].includes(field.mode)) return false
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
    || subscription.state !== 'ACTIVE' || subscription.pendingChangeId
    || subscription.salesScope !== salesScope) return false
  if (subscription.planCode !== target.planCode || receipt.after.planCode !== target.planCode
    || uint64(subscription.planVersion) === null || uint64(receipt.after.planVersion) === null
    || String(subscription.planVersion) !== String(target.version)
    || String(receipt.after.planVersion) !== String(target.version)) return false
  if (!notOlder(subscription.revision, receipt.after.revision)
    || !notOlder(subscription.entitlementSourceVersion, receipt.afterSourceVersion)
    || !notOlder(entitlements.sourceVersion, subscription.entitlementSourceVersion)
    || !notOlder(entitlements.sourceVersion, receipt.afterSourceVersion)
    || !notOlder(entitlements.entitlementVersion, receipt.afterEntitlementVersion)) return false
  return targetDecisionsConfirmed(entitlements, target)
}
