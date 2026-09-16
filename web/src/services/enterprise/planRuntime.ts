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

export type EnterpriseTenantUsage = Readonly<{
  usages: EnterpriseQuotaUsage[]
}>

export type EnterprisePlanSubscription = TenantSubscriptionDTO &
  Readonly<{
    updatedAt?: string
  }>

export type EnterprisePlanReadModel = Readonly<{
  session: TrustedSession
  subscription: EnterprisePlanSubscription
  entitlements: EntitlementView
  usage: EnterpriseTenantUsage
  usageError: string
}>

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

export function enterprisePlanRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有查看套餐与权益的权限。'
    if (error.code === 'conflict') return '租户上下文已变化，请刷新后重试。'
    return error.message
  }
  return error instanceof Error ? error.message : '套餐与权益服务请求失败。'
}

export async function readEnterprisePlanSession() {
  return readSession()
}

export async function getMyTenantSubscription(session: TrustedSession) {
  requireTenantSession(session)
  return request<TenantSubscriptionDTO>('/v1/tenant/subscription', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
}

export async function getMyTenantEntitlements(session: TrustedSession) {
  requireTenantSession(session)
  return mutate<EntitlementView>(
    '/v1/tenant/entitlements',
    'POST',
    { capabilityCodes: [] },
    { sessionContext: sessionContext(session) },
  )
}

export async function getMyTenantUsage(session: TrustedSession) {
  requireTenantSession(session)
  return request<EnterpriseTenantUsage>('/v1/tenant/usage', {
    headers: { 'X-Biz-Session-Context': sessionContext(session) },
  })
}

function isUsageAuthBoundaryError(error: unknown) {
  return (
    error instanceof CommercialApiError &&
    (error.code === 'unauthenticated' || error.code === 'forbidden' || error.code === 'conflict')
  )
}

export async function loadEnterprisePlanReadModel(): Promise<EnterprisePlanReadModel> {
  const session = await readEnterprisePlanSession()
  requireTenantSession(session)
  const [subscription, entitlements] = await Promise.all([
    getMyTenantSubscription(session),
    getMyTenantEntitlements(session),
  ])
  if (subscription.tenantId && subscription.tenantId !== session.active_tenant_id) {
    throw new Error('套餐服务返回了不属于当前租户的数据。')
  }
  if (entitlements.tenantId && entitlements.tenantId !== session.active_tenant_id) {
    throw new Error('权益服务返回了不属于当前租户的数据。')
  }

  let usage: EnterpriseTenantUsage = { usages: [] }
  let usageError = ''
  try {
    usage = await getMyTenantUsage(session)
  } catch (error) {
    if (isUsageAuthBoundaryError(error)) throw error
    usageError = enterprisePlanRuntimeError(error)
  }

  return Object.freeze({ session, subscription, entitlements, usage, usageError })
}
