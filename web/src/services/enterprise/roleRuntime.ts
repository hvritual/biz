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
  type PermissionGrant,
  type TenantRole,
  type TrustedSession,
} from '@/services/runtime/api'
import type { RoleGrantScope } from './rolePermissionCatalog'

export type RoleMutation = 'create' | 'update' | 'enable' | 'disable' | 'delete' | 'permissions' | 'assign' | 'revoke'

export type EnterpriseTenantRole = TenantRole & {
  permissions: PermissionGrant[]
  description: string
  roleCode: string
  systemRole: boolean
  memberCount: number
  protectedOwner: boolean
  protectedSystem: boolean
}

export type EnterpriseRoleDraft = {
  name: string
  description: string
  enabled: boolean
  permissions: PermissionGrant[]
  memberIds: string[]
}

export function roleRequestId(action: RoleMutation, suffix = '') {
  return commercialRequestId(`enterprise-role-${action}${suffix ? `-${suffix}` : ''}`)
}

export function roleStatusLabel(status: string) {
  return (
    {
      TENANT_ROLE_STATUS_ACTIVE: '已启用',
      TENANT_ROLE_STATUS_DISABLED: '已停用',
    }[status] ?? status
  )
}

export function roleRuntimeError(error: unknown) {
  if (error instanceof CommercialApiError) {
    if (error.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
    if (error.code === 'forbidden') return '当前账号没有管理企业角色与权限的权限。'
    if (error.code === 'conflict') return '角色版本或所有者保护规则已发生冲突，请刷新后重试。'
    return error.message
  }
  return error instanceof Error ? error.message : '角色权限服务请求失败。'
}

function requireTenantSession(session: TrustedSession) {
  if (!session.authenticated) throw new Error('请先登录业务账号。')
  if (!session.active_tenant_id) throw new Error('请选择可访问的租户。')
}

function headers(session: TrustedSession) {
  return { 'X-Biz-Session-Context': sessionContext(session) }
}

function normalizeScope(scope: string): RoleGrantScope {
  switch (scope) {
    case 'DATA_SCOPE_NONE': return 'none'
    case 'DATA_SCOPE_SELF': return 'self'
    case 'DATA_SCOPE_SITES': return 'sites'
    case 'DATA_SCOPE_ALL': return 'all'
    case 'none':
    case 'self':
    case 'sites':
    case 'all':
      return scope
    default:
      return 'none'
  }
}

function serverScope(scope: string) {
  switch (normalizeScope(scope)) {
    case 'self': return 'DATA_SCOPE_SELF'
    case 'sites': return 'DATA_SCOPE_SITES'
    case 'all': return 'DATA_SCOPE_ALL'
    default: return 'DATA_SCOPE_NONE'
  }
}

function roleSnapshot(role: TenantRole): EnterpriseTenantRole {
  const permissions = Array.isArray(role.permissions)
    ? role.permissions.map((grant) => Object.freeze({
        permission: grant.permission,
        scope: normalizeScope(grant.scope),
      }))
    : []
  const roleCode = role.roleCode ?? ''
  const systemRole = Boolean(role.systemRole)
  return Object.freeze({
    ...role,
    description: role.description ?? '',
    roleCode,
    systemRole,
    memberCount: Number(role.memberCount ?? 0),
    permissions,
    protectedOwner: roleCode === 'tenant_owner' || (!roleCode && role.name === 'owner'),
    protectedSystem: systemRole || roleCode === 'tenant_owner' || roleCode === 'tenant_admin',
  }) as EnterpriseTenantRole
}

export async function readEnterpriseRoleSession() {
  return readSession()
}

export async function switchEnterpriseRoleTenant(tenantId: string) {
  return selectSessionTenant(tenantId)
}

export async function listEnterpriseRoles(
  session: TrustedSession,
  filters: { query?: string; status?: string } = {},
) {
  requireTenantSession(session)
  const params = new URLSearchParams()
  if (filters.query?.trim()) params.set('query', filters.query.trim())
  if (filters.status) params.set('status', filters.status)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const result = await request<{ roles?: TenantRole[] }>(`/v1/tenant/roles${suffix}`, { headers: headers(session) })
  return Array.isArray(result.roles) ? result.roles.map(roleSnapshot) : []
}

export async function getEnterpriseRole(session: TrustedSession, roleId: string) {
  requireTenantSession(session)
  const role = await request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(roleId)}`, {
    headers: headers(session),
  })
  return roleSnapshot(role)
}

async function roleMutate(
  session: TrustedSession,
  path: string,
  method: 'POST' | 'PATCH' | 'PUT' | 'DELETE',
  body: unknown,
  idempotencyKey: string,
) {
  requireTenantSession(session)
  const role = await mutate<TenantRole>(path, method, body, {
    idempotencyKey,
    sessionContext: sessionContext(session),
  })
  return roleSnapshot(role)
}

export function createEnterpriseRole(session: TrustedSession, name: string, description: string, key: string) {
  return roleMutate(session, '/v1/tenant/roles', 'POST', { name: name.trim(), description: description.trim() }, key)
}

export function updateEnterpriseRole(
  session: TrustedSession,
  role: EnterpriseTenantRole,
  name: string,
  description: string,
  key: string,
) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(role.id)}`,
    'PATCH',
    { roleId: role.id, name: name.trim(), description: description.trim(), version: role.version },
    key,
  )
}

export function deleteEnterpriseRole(session: TrustedSession, role: EnterpriseTenantRole, key: string) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(role.id)}?version=${encodeURIComponent(String(role.version))}`,
    'DELETE',
    {},
    key,
  )
}

export function enableEnterpriseRole(session: TrustedSession, role: EnterpriseTenantRole, key: string) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(role.id)}/enable`,
    'POST',
    { roleId: role.id, version: role.version },
    key,
  )
}

export function disableEnterpriseRole(session: TrustedSession, role: EnterpriseTenantRole, key: string) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(role.id)}/disable`,
    'POST',
    { roleId: role.id, version: role.version },
    key,
  )
}

export function setEnterpriseRolePermissions(
  session: TrustedSession,
  role: EnterpriseTenantRole,
  permissions: PermissionGrant[],
  key: string,
) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(role.id)}/permissions`,
    'PUT',
    {
      roleId: role.id,
      version: role.version,
      permissions: permissions.map((grant) => ({ permission: grant.permission, scope: serverScope(grant.scope) })),
    },
    key,
  )
}

export function assignEnterpriseRoleMember(
  session: TrustedSession,
  roleId: string,
  userId: string,
  key: string,
) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(roleId)}/members`,
    'POST',
    { roleId, userId },
    key,
  )
}

export function revokeEnterpriseRoleMember(
  session: TrustedSession,
  roleId: string,
  userId: string,
  key: string,
) {
  return roleMutate(
    session,
    `/v1/tenant/roles/${encodeURIComponent(roleId)}/members/${encodeURIComponent(userId)}/revoke`,
    'POST',
    { roleId, userId },
    key,
  )
}
