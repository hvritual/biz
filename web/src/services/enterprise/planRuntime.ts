import {
  CommercialApiError,
  mutate,
  request,
  type EntitlementView,
  type TenantSubscriptionDTO,
} from '@/services/commercial/platformCommercial'
import { readSession, sessionContext, type TrustedSession } from '@/services/runtime/api'

export type EnterprisePlanReadModel = Readonly<{
  session: TrustedSession
  subscription: TenantSubscriptionDTO
  entitlements: EntitlementView
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
  return Object.freeze({ session, subscription, entitlements })
}
