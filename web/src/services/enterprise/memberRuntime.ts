import {
  CommercialApiError,
  commercialRequestId,
  mutate,
  request,
} from '@/services/commercial/platformCommercial'
import {
  readSession,
  selectSessionTenant,
  sessionContext,
  type TenantMember,
  type TrustedSession,
} from '@/services/runtime/api'

export type MemberMutation = 'invite' | 'activate' | 'suspend' | 'remove'

export function memberRequestId(action: MemberMutation) {
  return commercialRequestId(`enterprise-member-${action}`)
}

export function sameTrustedSession(a: TrustedSession, b: TrustedSession) {
  return (
    a.authenticated === b.authenticated &&
    (a.actor_kind ?? '') === (b.actor_kind ?? '') &&
    (a.user_id ?? '') === (b.user_id ?? '') &&
    (a.active_tenant_id ?? '') === (b.active_tenant_id ?? '')
  )
}

export function memberStatusLabel(status: string) {
  return (
    {
      TENANT_MEMBER_STATUS_INVITED: '待激活',
      TENANT_MEMBER_STATUS_ACTIVE: '已启用',
      TENANT_MEMBER_STATUS_SUSPENDED: '已停用',
      TENANT_MEMBER_STATUS_REMOVED: '已移除',
    }[status] ?? status
  )
}

export function memberRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有管理企业成员的权限。'
    if (error.code === 'conflict') return '成员状态或请求版本已发生变化，请刷新后重试。'
    return error.message
  }
  return error instanceof Error ? error.message : '成员服务请求失败。'
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function readHeaders(session: TrustedSession) {
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

// Server DTOs are snapshots, not mutable form models. Freezing keeps Vue from
// wrapping them in reactive proxies and makes copies safe for operation drafts.
function memberSnapshot(member: TenantMember): TenantMember {
  return Object.freeze({ ...member })
}

export async function readEnterpriseMemberSession() {
  return readSession()
}

export async function switchEnterpriseMemberTenant(tenantId: string) {
  return selectSessionTenant(tenantId)
}

export async function listEnterpriseMembers(session: TrustedSession) {
  requireTenantSession(session)
  const result = await request<{ members?: TenantMember[] }>('/v1/tenant/members', {
    headers: readHeaders(session),
  })
  return Array.isArray(result.members) ? result.members.map(memberSnapshot) : []
}

export async function getEnterpriseMember(session: TrustedSession, userId: string) {
  requireTenantSession(session)
  const member = await request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(userId)}`, {
    headers: readHeaders(session),
  })
  return memberSnapshot(member)
}

async function memberMutate(
  session: TrustedSession,
  path: string,
  body: unknown,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const member = await mutate<TenantMember>(path, 'POST', body, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return memberSnapshot(member)
}

export function inviteEnterpriseMember(session: TrustedSession, email: string, idempotencyKey: string) {
  return memberMutate(session, '/v1/tenant/members', { email: email.trim() }, idempotencyKey)
}

export function activateEnterpriseMember(
  session: TrustedSession,
  member: TenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/activate`,
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}

export function suspendEnterpriseMember(
  session: TrustedSession,
  member: TenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/suspend`,
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}

export function removeEnterpriseMember(
  session: TrustedSession,
  member: TenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/remove`,
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}
