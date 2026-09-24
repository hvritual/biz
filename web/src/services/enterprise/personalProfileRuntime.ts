import { backendErrorFallback } from '@/i18n/backend-terms'
import {
  CommercialApiError,
  commercialRequestId,
  mutate,
  request,
} from '@/services/commercial/platformCommercial'
import {
  readSession,
  sessionContext,
  type TrustedSession,
} from '@/services/runtime/api'

export type PersonalProfileRole = Readonly<{
  roleId: string
  roleName: string
  roleStatus: string
}>

export type TenantPersonalProfile = Readonly<{
  userId: string
  username: string
  name: string
  tenantId: string
  tenantName: string
  roles: readonly PersonalProfileRole[]
  registeredAt: string
  joinedAt: string
  email: string
  phone: string
  avatarAssetRef: string
  version: string | number
  employeeId: string
  position: string
  departmentId: string
}>

export type PersonalAvatarOption = Readonly<{
  assetRef: string
  name: string
  tone: string
}>

export function personalProfileRequestId(tenantId: string) {
  return commercialRequestId(`enterprise-personal-profile-avatar-${tenantId}`)
}

export function personalProfileRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有查看或维护本人资料的权限。'
    if (error.code === 'conflict') return '个人资料版本已变化，请重新读取后再试。'
    return backendErrorFallback('member')
  }
  return error instanceof Error ? error.message : backendErrorFallback('member')
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated || session.actor_kind !== 'tenant') throw new Error('请先登录业务账号。')
  if (!session.user_id) throw new Error('当前会话缺少可信用户身份。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function readHeaders(session: TrustedSession) {
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

function safeAvatarAssetRef(value: unknown) {
  const assetRef = typeof value === 'string' ? value.trim() : ''
  if (!assetRef.startsWith('avatar:')) return ''
  if (assetRef.includes('://') || assetRef.startsWith('data:') || assetRef.startsWith('javascript:')) return ''
  return assetRef
}

function profileSnapshot(value: Partial<TenantPersonalProfile>): TenantPersonalProfile {
  const roles = Array.isArray(value.roles)
    ? value.roles.map((role) => Object.freeze({
        roleId: role.roleId ?? '',
        roleName: role.roleName ?? '',
        roleStatus: role.roleStatus ?? '',
      }))
    : []
  return Object.freeze({
    userId: value.userId ?? '',
    username: value.username ?? '',
    name: value.name ?? '',
    tenantId: value.tenantId ?? '',
    tenantName: value.tenantName ?? '',
    roles,
    registeredAt: value.registeredAt ?? '',
    joinedAt: value.joinedAt ?? '',
    email: value.email ?? '',
    phone: value.phone ?? '',
    avatarAssetRef: safeAvatarAssetRef(value.avatarAssetRef) || 'avatar:coffee-blue',
    version: value.version ?? 0,
    employeeId: value.employeeId ?? '',
    position: value.position ?? '',
    departmentId: value.departmentId ?? '',
  })
}

function avatarOptionSnapshot(value: Partial<PersonalAvatarOption>): PersonalAvatarOption | null {
  const assetRef = safeAvatarAssetRef(value.assetRef)
  if (!assetRef) return null
  return Object.freeze({
    assetRef,
    name: value.name ?? assetRef,
    tone: value.tone ?? '',
  })
}

function assertSelfProfile(session: TrustedSession, profile: TenantPersonalProfile) {
  if (profile.tenantId !== session.active_tenant_id || profile.userId !== session.user_id) {
    throw new Error('个人资料与当前可信会话不匹配，请刷新后重试。')
  }
}

export function samePersonalProfileSession(a: TrustedSession, b: TrustedSession) {
  return (
    a.authenticated === b.authenticated &&
    (a.actor_kind ?? '') === (b.actor_kind ?? '') &&
    (a.user_id ?? '') === (b.user_id ?? '') &&
    (a.active_tenant_id ?? '') === (b.active_tenant_id ?? '') &&
    (a.context_version ?? 0) === (b.context_version ?? 0)
  )
}

export async function readPersonalProfileSession() {
  return readSession()
}

export async function getMyPersonalProfile(session: TrustedSession) {
  requireTenantSession(session)
  const value = await request<Partial<TenantPersonalProfile>>('/v1/tenant/me/profile', {
    headers: readHeaders(session),
  })
  const profile = profileSnapshot(value)
  assertSelfProfile(session, profile)
  return profile
}

export async function listMyPersonalAvatarOptions(session: TrustedSession) {
  requireTenantSession(session)
  const value = await request<{ options?: Partial<PersonalAvatarOption>[] }>('/v1/tenant/me/avatar-options', {
    headers: readHeaders(session),
  })
  return Object.freeze(
    (value.options ?? [])
      .map(avatarOptionSnapshot)
      .filter((option): option is PersonalAvatarOption => Boolean(option)),
  )
}

export async function updateMyPersonalAvatar(
  session: TrustedSession,
  current: TenantPersonalProfile,
  avatarAssetRef: string,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  assertSelfProfile(session, current)
  const safeRef = safeAvatarAssetRef(avatarAssetRef)
  if (!safeRef) throw new Error('请选择服务端允许的头像。')
  await mutate<TenantPersonalProfile>(
    '/v1/tenant/me/avatar',
    'PATCH',
    { version: current.version, avatarAssetRef: safeRef },
    { idempotencyKey, sessionContext: sessionContext(session) },
  )
  return getMyPersonalProfile(session)
}
