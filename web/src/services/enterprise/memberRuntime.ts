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
  type TenantRole,
  type TrustedSession,
} from '@/services/runtime/api'

export type MemberMutation = 'invite' | 'activate' | 'suspend' | 'remove' | 'profile' | 'roles'
export type MemberRoleMutation = 'assign' | 'revoke'

export type EnterpriseMemberRole = {
  roleId: string
  roleName: string
  roleStatus: string
}

export type EnterpriseTenantMember = TenantMember & {
  name: string
  phone: string
  employeeId: string
  position: string
  departmentId: string
  roles: EnterpriseMemberRole[]
  derivedDataScope: string
}

export type EnterpriseMemberProfileInput = {
  name: string
  phone: string
  employeeId: string
  position: string
  departmentId: string
}

export function memberRequestId(action: MemberMutation) {
  return commercialRequestId(`enterprise-member-${action}`)
}

export function memberRoleRequestId(action: MemberRoleMutation) {
  return commercialRequestId(`enterprise-member-role-${action}`)
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

export function memberDataScopeLabel(scope: string) {
  return (
    {
      none: '无数据权限',
      self: '仅本人',
      sites: '授权点位',
      all: '全部数据',
    }[scope] ?? (scope || '无数据权限')
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

// Server DTOs are immutable read-model snapshots. Nested role summaries are
// copied and frozen as well so UI form drafts never mutate authoritative state.
function memberSnapshot(member: TenantMember & Partial<EnterpriseTenantMember>): EnterpriseTenantMember {
  const roles = Array.isArray(member.roles)
    ? member.roles.map((role) => Object.freeze({ ...role }))
    : []
  return Object.freeze({
    ...member,
    name: member.name ?? '',
    phone: member.phone ?? '',
    employeeId: member.employeeId ?? '',
    position: member.position ?? '',
    departmentId: member.departmentId ?? '',
    roles,
    derivedDataScope: member.derivedDataScope ?? 'none',
  }) as EnterpriseTenantMember
}

function roleSnapshot(role: TenantRole): TenantRole {
  return Object.freeze({
    ...role,
    permissions: Array.isArray(role.permissions)
      ? role.permissions.map((grant) => Object.freeze({ ...grant }))
      : [],
  })
}

export async function readEnterpriseMemberSession() {
  return readSession()
}

export async function switchEnterpriseMemberTenant(tenantId: string) {
  return selectSessionTenant(tenantId)
}

export async function listEnterpriseMembers(session: TrustedSession) {
  requireTenantSession(session)
  const result = await request<{ members?: Array<TenantMember & Partial<EnterpriseTenantMember>> }>(
    '/v1/tenant/members',
    { headers: readHeaders(session) },
  )
  return Array.isArray(result.members) ? result.members.map(memberSnapshot) : []
}

export async function getEnterpriseMember(session: TrustedSession, userId: string) {
  requireTenantSession(session)
  const member = await request<TenantMember & Partial<EnterpriseTenantMember>>(
    `/v1/tenant/members/${encodeURIComponent(userId)}`,
    { headers: readHeaders(session) },
  )
  return memberSnapshot(member)
}

export async function listEnterpriseRoles(session: TrustedSession) {
  requireTenantSession(session)
  const result = await request<{ roles?: TenantRole[] }>('/v1/tenant/roles', {
    headers: readHeaders(session),
  })
  return Array.isArray(result.roles) ? result.roles.map(roleSnapshot) : []
}

async function memberMutate(
  session: TrustedSession,
  path: string,
  method: 'POST' | 'PATCH',
  body: unknown,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const member = await mutate<TenantMember & Partial<EnterpriseTenantMember>>(path, method, body, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return memberSnapshot(member)
}

export function inviteEnterpriseMember(session: TrustedSession, email: string, idempotencyKey: string) {
  return memberMutate(session, '/v1/tenant/members', 'POST', { email: email.trim() }, idempotencyKey)
}

export function updateEnterpriseMemberProfile(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  input: EnterpriseMemberProfileInput,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/profile`,
    'PATCH',
    {
      userId: member.userId,
      name: input.name.trim(),
      phone: input.phone.trim(),
      employeeId: input.employeeId.trim(),
      position: input.position.trim(),
      departmentId: input.departmentId.trim(),
      version: member.version,
    },
    idempotencyKey,
  )
}

export function activateEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/activate`,
    'POST',
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}

export function suspendEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/suspend`,
    'POST',
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}

export function removeEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/remove`,
    'POST',
    { userId: member.userId, version: member.version },
    idempotencyKey,
  )
}

export async function assignEnterpriseMemberRole(
  session: TrustedSession,
  userId: string,
  roleId: string,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  return mutate<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(roleId)}/members`, 'POST', {
    roleId,
    userId,
  }, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
}

export async function revokeEnterpriseMemberRole(
  session: TrustedSession,
  userId: string,
  roleId: string,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  return mutate<TenantRole>(
    `/v1/tenant/roles/${encodeURIComponent(roleId)}/members/${encodeURIComponent(userId)}/revoke`,
    'POST',
    { roleId, userId },
    {
      idempotencyKey,
      sessionContext: sessionContext(session),
    },
  )
}
