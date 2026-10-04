import { t } from '@/i18n'
import { backendErrorFallback } from '@/i18n/backend-terms'
import {
  CommercialApiError,
  mutate,
  request,
  type EntitlementView,
  type TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'

export type EnterpriseQuotaUsage = Readonly<{
  moduleCode: string
  key: string
  known: boolean
  used: string | number
  evidence: string
}>
export type EnterpriseTenantUsage = Readonly<{ usages: EnterpriseQuotaUsage[] }>
export type EnterprisePlanSubscription = TenantSubscriptionDTO & Readonly<{ updatedAt?: string }>
export type EnterprisePlanReadModel = Readonly<{
  session: TrustedSession
  subscription: EnterprisePlanSubscription
  entitlements: EntitlementView
  usage: EnterpriseTenantUsage
  usageError: string
}>
export type PlanReadIssue = 'login' | 'denied' | 'context' | 'unavailable' | 'usage'

export function enterprisePlanReadIssue(error: unknown): PlanReadIssue {
  if (error instanceof CommercialApiError) {
    if (error.status === 401) return 'login'
    if (error.status === 403) return 'denied'
    if (error.status === 409) return 'context'
  }
  return 'unavailable'
}

export function enterprisePlanRuntimeError(error: unknown) {
  const issue = enterprisePlanReadIssue(error)
  if (issue === 'login') return t('planFeedback.access.login')
  if (issue === 'denied') return t('planFeedback.readDeniedBody')
  if (issue === 'context') return t('planFeedback.contextChangedBody')
  // Never render arbitrary transport messages, stack traces, or HTML as copy.
  return backendErrorFallback('planRead')
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new CommercialApiError('Unauthenticated plan read', 401, 'unauthenticated')
  if (!session.active_tenant_id) throw new CommercialApiError('No active tenant', 409, 'conflict')
}

export async function readEnterprisePlanSession() { return readSession() }

export async function getMyTenantSubscription(session: TrustedSession) {
  requireTenantSession(session)
  return request<TenantSubscriptionDTO>('/v1/tenant/subscription', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
}

export async function getMyTenantEntitlements(session: TrustedSession) {
  requireTenantSession(session)
  return mutate<EntitlementView>(
    '/v1/tenant/entitlements', 'POST', { capabilityCodes: [] },
    { sessionContext: sessionContext(session) },
  )
}

export async function getMyTenantUsage(session: TrustedSession) {
  requireTenantSession(session)
  return request<EnterpriseTenantUsage>('/v1/tenant/usage', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
}

export async function loadEnterprisePlanReadModel(expectedSession?: TrustedSession): Promise<EnterprisePlanReadModel> {
  const session = await readEnterprisePlanSession()
  requireTenantSession(session)
  if (expectedSession && sessionContext(expectedSession) !== sessionContext(session)) {
    throw new CommercialApiError('Plan session changed', 409, 'conflict')
  }
  const [subscription, entitlements] = await Promise.all([
    getMyTenantSubscription(session), getMyTenantEntitlements(session),
  ])
  if (subscription.tenantId !== session.active_tenant_id || entitlements.tenantId !== session.active_tenant_id) {
    throw new CommercialApiError('Plan response scope mismatch', 409, 'conflict')
  }
  if (!Array.isArray(entitlements.decisions)) throw new Error('Invalid entitlement response')

  let usage: EnterpriseTenantUsage = { usages: [] }
  let usageError = ''
  try {
    usage = await getMyTenantUsage(session)
    if (!Array.isArray(usage.usages)) throw new Error('Invalid usage response')
  } catch (error) {
    if (enterprisePlanReadIssue(error) !== 'unavailable') throw error
    usage = { usages: [] }
    usageError = enterprisePlanRuntimeError(error)
  }
  return Object.freeze({ session, subscription, entitlements, usage, usageError })
}
