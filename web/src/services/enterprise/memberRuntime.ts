import { backendErrorFallback, backendTermLabel } from '@/i18n/backend-terms'
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

export type MemberMutation = 'create' | 'update' | 'invite' | 'activate' | 'suspend' | 'remove' | 'restore' | 'profile' | 'roles' | 'scope'
export type MemberRoleMutation = 'assign' | 'revoke'

export type EnterpriseMemberScopeCandidate = {
  id: string
  name: string
  version: number
  assignable: boolean
  unavailableReason: string
}

export type EnterpriseMemberBusinessScope = {
  userId: string
  version: number
  siteIds: string[]
  tenantId: string
}

export type EnterpriseMemberRole = {
  roleId: string
  roleName: string
  roleStatus: string
}

export type EnterpriseTenantMember = TenantMember & {
  username: string
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

export type EnterpriseMemberActivationMode = 'activation_link' | 'sms_initial_password'

export type EnterpriseMemberCreateInput = EnterpriseMemberProfileInput & {
  username: string
  email: string
  roleIds: string[]
  activationMode: EnterpriseMemberActivationMode
}

export type EnterpriseMemberUpdateInput = EnterpriseMemberProfileInput & {
  email: string
  roleIds: string[]
}

export type EnterpriseMemberCreationReceipt = {
  member: EnterpriseTenantMember
  activationMode: string
  notificationEventId: string
  deliveryState: string
  maskedDestination: string
}

export type EnterpriseMemberListQuery = {
  query?: string
  roleId?: string
  departmentId?: string
  status?: string
  page: number
  pageSize: number
}

export type EnterpriseMemberListPage = {
  members: EnterpriseTenantMember[]
  total: number
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
    (a.active_tenant_id ?? '') === (b.active_tenant_id ?? '') &&
    (a.context_version ?? 0) === (b.context_version ?? 0)
  )
}

export function memberStatusLabel(status: string) {
  return backendTermLabel('memberStatus', status)
}

export function memberDataScopeLabel(scope: string) {
  return backendTermLabel('dataScope', scope)
}

export function memberRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有管理企业成员的权限。'
    if (error.code === 'conflict') {
      const message = error.message.toLowerCase()
      if (message.includes('username')) return '该登录账号已被其他 Account 使用，请更换账号。'
      if (message.includes('contact')) return '该手机号或邮箱已被其他成员使用，请核对联系方式。'
      if (message.includes('activation')) return '该成员仍有待完成的激活流程，不能由管理员直接启用。'
      return '成员状态或请求版本已发生变化，请刷新后重试。'
    }
    return backendErrorFallback('member')
  }
  return error instanceof Error ? error.message : backendErrorFallback('member')
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
    username: member.username ?? '',
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

export async function queryEnterpriseMembers(
  session: TrustedSession,
  input: EnterpriseMemberListQuery,
): Promise<EnterpriseMemberListPage> {
  requireTenantSession(session)
  const params = new URLSearchParams()
  const query = input.query?.trim() ?? ''
  const roleId = input.roleId?.trim() ?? ''
  const departmentId = input.departmentId?.trim() ?? ''
  const status = input.status?.trim() ?? ''
  if (query) params.set('query', query)
  if (roleId) params.set('role_id', roleId)
  if (departmentId) params.set('department_id', departmentId)
  if (status) params.set('status', status)
  params.set('page', String(Math.max(1, Math.trunc(input.page))))
  params.set('page_size', String(Math.max(1, Math.min(100, Math.trunc(input.pageSize)))))
  const result = await request<{
    members?: Array<TenantMember & Partial<EnterpriseTenantMember>>
    total?: string | number
  }>(`/v1/tenant/members?${params.toString()}`, { headers: readHeaders(session) })
  const total = Number(result.total ?? 0)
  return {
    members: Array.isArray(result.members) ? result.members.map(memberSnapshot) : [],
    total: Number.isFinite(total) && total >= 0 ? total : 0,
  }
}

export async function listRemovedEnterpriseMembers(
  session: TrustedSession,
  page = 1,
  pageSize = 50,
): Promise<EnterpriseMemberListPage> {
  requireTenantSession(session)
  const params = new URLSearchParams({
    page: String(Math.max(1, Math.trunc(page))),
    page_size: String(Math.max(1, Math.min(100, Math.trunc(pageSize)))),
  })
  const result = await request<{
    members?: Array<TenantMember & Partial<EnterpriseTenantMember>>
    total?: string | number
  }>(`/v1/tenant/members/removed?${params.toString()}`, { headers: readHeaders(session) })
  const total = Number(result.total ?? 0)
  return {
    members: Array.isArray(result.members) ? result.members.map(memberSnapshot) : [],
    total: Number.isFinite(total) && total >= 0 ? total : 0,
  }
}

export async function listEnterpriseMembers(session: TrustedSession) {
  const members: EnterpriseTenantMember[] = []
  let page = 1
  const pageSize = 100
  for (;;) {
    const result = await queryEnterpriseMembers(session, { page, pageSize })
    members.push(...result.members)
    if (members.length >= result.total || result.members.length === 0) return members
    page += 1
  }
}

export async function getEnterpriseMember(session: TrustedSession, userId: string) {
  requireTenantSession(session)
  const member = await request<TenantMember & Partial<EnterpriseTenantMember>>(
    `/v1/tenant/members/${encodeURIComponent(userId)}`,
    { headers: readHeaders(session) },
  )
  return memberSnapshot(member)
}

export async function listEnterpriseMemberScopeCandidates(
  session: TrustedSession,
  input: { query?: string; page?: number; pageSize?: number } = {},
) {
  requireTenantSession(session)
  const params = new URLSearchParams()
  if (input.query?.trim()) params.set('query', input.query.trim())
  params.set('page', String(Math.max(1, Math.trunc(input.page ?? 1))))
  params.set('page_size', String(Math.max(1, Math.min(100, Math.trunc(input.pageSize ?? 100)))))
  const result = await request<{
    candidates?: Array<{ id: string; name?: string; version?: string | number; assignable?: boolean; unavailableReason?: string }>
    total?: string | number
  }>(`/v1/tenant/member-scope-candidates?${params.toString()}`, { headers: readHeaders(session) })
  return {
    candidates: (result.candidates ?? []).map((candidate) => Object.freeze({
      id: candidate.id,
      name: candidate.name ?? candidate.id,
      version: Number(candidate.version ?? 0),
      assignable: candidate.assignable !== false,
      unavailableReason: candidate.unavailableReason ?? '',
    }) as EnterpriseMemberScopeCandidate),
    total: Number(result.total ?? 0),
  }
}

export async function getEnterpriseMemberBusinessScope(session: TrustedSession, userId: string) {
  requireTenantSession(session)
  const value = await request<{ userId: string; version: string | number; siteIds?: string[]; tenantId: string }>(
    `/v1/tenant/members/${encodeURIComponent(userId)}/business-scope`,
    { headers: readHeaders(session) },
  )
  return Object.freeze({
    userId: value.userId,
    version: Number(value.version ?? 0),
    siteIds: [...(value.siteIds ?? [])].sort(),
    tenantId: value.tenantId,
  }) as EnterpriseMemberBusinessScope
}

export async function setEnterpriseMemberBusinessScope(
  session: TrustedSession,
  current: EnterpriseMemberBusinessScope,
  siteIds: string[],
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const value = await mutate<{ userId: string; version: string | number; siteIds?: string[]; tenantId: string }>(
    `/v1/tenant/members/${encodeURIComponent(current.userId)}/business-scope`,
    'PUT',
    { userId: current.userId, version: current.version, siteIds: [...new Set(siteIds)].sort() },
    { idempotencyKey, sessionContext: sessionContext(session) },
  )
  return Object.freeze({
    userId: value.userId,
    version: Number(value.version ?? 0),
    siteIds: [...(value.siteIds ?? [])].sort(),
    tenantId: value.tenantId,
  }) as EnterpriseMemberBusinessScope
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

function activationModeWire(mode: EnterpriseMemberActivationMode) {
  return mode === 'sms_initial_password'
    ? 'TENANT_MEMBER_ACTIVATION_MODE_SMS_INITIAL_PASSWORD'
    : 'TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK'
}

export async function createEnterpriseMember(
  session: TrustedSession,
  input: EnterpriseMemberCreateInput,
  idempotencyKey: string,
): Promise<EnterpriseMemberCreationReceipt> {
  requireTenantSession(session)
  const result = await mutate<{
    member: TenantMember & Partial<EnterpriseTenantMember>
    activationMode?: string
    notificationEventId?: string
    deliveryState?: string
    maskedDestination?: string
  }>('/v1/tenant/members/create', 'POST', {
    username: input.username.trim().toLowerCase(),
    email: input.email.trim(),
    phone: input.phone.trim(),
    name: input.name.trim(),
    employeeId: input.employeeId.trim(),
    position: input.position.trim(),
    departmentId: input.departmentId.trim(),
    roleIds: [...input.roleIds],
    activationMode: activationModeWire(input.activationMode),
  }, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return {
    member: memberSnapshot(result.member),
    activationMode: result.activationMode ?? '',
    notificationEventId: result.notificationEventId ?? '',
    deliveryState: result.deliveryState ?? '',
    maskedDestination: result.maskedDestination ?? '',
  }
}

export function updateEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  input: EnterpriseMemberUpdateInput,
  idempotencyKey: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}`,
    'PATCH',
    {
      userId: member.userId,
      email: input.email.trim(),
      phone: input.phone.trim(),
      name: input.name.trim(),
      employeeId: input.employeeId.trim(),
      position: input.position.trim(),
      departmentId: input.departmentId.trim(),
      roleIds: [...input.roleIds],
      version: member.version,
    },
    idempotencyKey,
  )
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
  reason = '',
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/activate`,
    'POST',
    { userId: member.userId, version: member.version, reason: reason.trim() },
    idempotencyKey,
  )
}

export function suspendEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
  reason = '',
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/suspend`,
    'POST',
    { userId: member.userId, version: member.version, reason: reason.trim() },
    idempotencyKey,
  )
}

export function removeEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
  reason = '',
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/remove`,
    'POST',
    { userId: member.userId, version: member.version, reason: reason.trim() },
    idempotencyKey,
  )
}

export function restoreEnterpriseMember(
  session: TrustedSession,
  member: EnterpriseTenantMember,
  idempotencyKey: string,
  reason: string,
) {
  return memberMutate(
    session,
    `/v1/tenant/members/${encodeURIComponent(member.userId)}/restore`,
    'POST',
    { userId: member.userId, version: member.version, reason: reason.trim() },
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
