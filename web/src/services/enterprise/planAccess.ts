import type { TrustedSession } from '@/services/runtime/api'

export type PlanAccess = 'allowed' | 'checking' | 'login' | 'iam' | 'entitlement' | 'unknown'

// A read-only projection of the existing authorization response, not a new policy source.
type PlanAuthorization = {
  authenticated: boolean
  tenant_id?: string
  user_id?: string
  button_codes: readonly string[]
  grants: readonly { permission: string }[]
  modules: readonly { allowed: boolean; actions: readonly string[] }[]
}

export function planChangeAccess(
  snapshot: PlanAuthorization | null,
  session: TrustedSession | null,
  status: string,
  contextCurrent: boolean,
): PlanAccess {
  if (status === 'unauthenticated') return 'login'
  if (!contextCurrent || status === 'idle' || status === 'loading') return 'checking'
  if (status !== 'ready' || !snapshot || !session?.authenticated) return 'unknown'
  if (!snapshot.authenticated || snapshot.tenant_id !== session.active_tenant_id || snapshot.user_id !== session.user_id) return 'unknown'
  if (!Array.isArray(snapshot.button_codes) || !Array.isArray(snapshot.modules)) return 'unknown'
  const operation = 'commercial.subscription.change.targets_my'
  const unavailable = snapshot.modules.some((module) =>
    module.allowed === false && Array.isArray(module.actions) && module.actions.includes(operation),
  )
  if (unavailable) return 'entitlement'
  if (snapshot.button_codes.includes(operation)) return 'allowed'
  if (!Array.isArray(snapshot.grants)) return 'unknown'
  // Both permissions are required by ListMySubscriptionChangeTargets. Their
  // presence cannot grant an operation absent from the server's button_codes.
  const permissions = new Set(snapshot.grants.map((grant) => grant.permission))
  if (!permissions.has('tenant.subscription.manage') || !permissions.has('commercial.catalog.read')) return 'iam'
  return 'unknown'
}

export function knownQuotaNumber(value: unknown): number | null {
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (typeof value === 'string' && value.trim() === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}
