import { request as read, mutate } from '@/services/commercial/platformCommercial'
export interface TrustedSession {
  authenticated: boolean
  actor_kind?: string
  platform_subject?: string
  user_id?: string
  active_tenant_id?: string
  tenants?: Array<{ id: string; name: string }>
}
export const readSession = () => read<TrustedSession>('/auth/session')
export const selectSessionTenant = (tenantId: string) =>
  mutate<TrustedSession>('/auth/session/tenant', 'POST', { tenant_id: tenantId })
export const logoutSession = () => mutate<void>('/auth/logout', 'POST', {})
export function loginUrl() {
  return `${(import.meta.env.VITE_API_BASE_URL ?? '/api').replace(/\/$/, '')}/auth/login?return_to=${encodeURIComponent(window.location.pathname + window.location.hash)}`
}
export type Tenant = {
  id: string
  name: string
  status: string
  version: string | number
}

export type TenantMember = {
  userId: string
  email: string
  status: string
  version: string | number
}

export type PermissionGrant = {
  permission: string
  scope: string
}

export type TenantRole = {
  id: string
  name: string
  status: string
  permissions?: PermissionGrant[]
  version: string | number
}

export type Device = {
  id: string
  siteId: string
  name: string
  serial: string
  createdBy: string
  version: string | number
}

export function sessionContext(s: TrustedSession) {
  return JSON.stringify({
    actor_kind: s.actor_kind ?? '',
    platform_subject: s.platform_subject ?? '',
    user_id: s.user_id ?? '',
    active_tenant_id: s.active_tenant_id ?? '',
  })
}
export function createRuntimeApi(requestId: string, expectedSession?: TrustedSession) {
  async function request<T>(
    path: string,
    options: { method?: 'POST' | 'PATCH' | 'PUT' | 'DELETE'; body?: unknown; idempotent?: boolean },
  ): Promise<T> {
    if (!options.method)
      return read<T>(path, {
        headers: expectedSession ? { 'X-Biz-Session-Context': sessionContext(expectedSession) } : {},
      })
    return mutate<T>(path, options.method, options.body ?? {}, {
      idempotencyKey: options.idempotent ? requestId : undefined,
      sessionContext: expectedSession ? sessionContext(expectedSession) : undefined,
    })
  }
  const platformApi = {
    async listTenants() {
      const result = await request<{ tenants?: Tenant[] }>('/v1/tenants', {})
      return result.tenants || []
    },
    createTenant(input: {
      name: string
      ownerUserId: string
      ownerEmail: string
      requestId: string
      salesScope: string
    }) {
      return request<Tenant>('/v1/tenants', {
        method: 'POST',
        body: input,
        idempotent: true,
      })
    },
    updateTenant(tenant: Tenant, name: string) {
      return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}`, {
        method: 'PATCH',
        body: { id: tenant.id, name, version: tenant.version },
        idempotent: true,
      })
    },
    activateTenant(tenant: Tenant) {
      return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/activate`, {
        method: 'POST',
        body: { id: tenant.id, version: tenant.version },
        idempotent: true,
      })
    },
    suspendTenant(tenant: Tenant) {
      return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/suspend`, {
        method: 'POST',
        body: { id: tenant.id, version: tenant.version },
        idempotent: true,
      })
    },
    closeTenant(tenant: Tenant) {
      return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/close`, {
        method: 'POST',
        body: { id: tenant.id, version: tenant.version },
        idempotent: true,
      })
    },
  }

  const tenantApi = {
    async listMembers() {
      const result = await request<{ members?: TenantMember[] }>('/v1/tenant/members', {})
      return result.members || []
    },
    inviteMember(email: string) {
      return request<TenantMember>('/v1/tenant/members', {
        method: 'POST',
        body: { email },
        idempotent: true,
      })
    },
    activateMember(member: TenantMember) {
      return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/activate`, {
        method: 'POST',
        body: { userId: member.userId, version: member.version },
        idempotent: true,
      })
    },
    suspendMember(member: TenantMember) {
      return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/suspend`, {
        method: 'POST',
        body: { userId: member.userId, version: member.version },
        idempotent: true,
      })
    },
    removeMember(member: TenantMember) {
      return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/remove`, {
        method: 'POST',
        body: { userId: member.userId, version: member.version },
        idempotent: true,
      })
    },
    async listRoles() {
      const result = await request<{ roles?: TenantRole[] }>('/v1/tenant/roles', {})
      return result.roles || []
    },
    createRole(name: string) {
      return request<TenantRole>('/v1/tenant/roles', {
        method: 'POST',
        body: { name },
        idempotent: true,
      })
    },
    updateRole(role: TenantRole, name: string) {
      return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}`, {
        method: 'PATCH',
        body: { roleId: role.id, name, version: role.version },
        idempotent: true,
      })
    },
    enableRole(role: TenantRole) {
      return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/enable`, {
        method: 'POST',
        body: { roleId: role.id, version: role.version },
        idempotent: true,
      })
    },
    disableRole(role: TenantRole) {
      return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/disable`, {
        method: 'POST',
        body: { roleId: role.id, version: role.version },
        idempotent: true,
      })
    },
    setRolePermissions(role: TenantRole, permissions: PermissionGrant[]) {
      return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/permissions`, {
        method: 'PUT',
        body: { roleId: role.id, permissions, version: role.version },
        idempotent: true,
      })
    },
    assignRoleMember(roleId: string, userId: string) {
      return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(roleId)}/members`, {
        method: 'POST',
        body: { roleId, userId },
        idempotent: true,
      })
    },
    revokeRoleMember(roleId: string, userId: string) {
      return request<TenantRole>(
        `/v1/tenant/roles/${encodeURIComponent(roleId)}/members/${encodeURIComponent(userId)}/revoke`,
        {
          method: 'POST',
          body: { roleId, userId },
          idempotent: true,
        },
      )
    },
    async listDevices() {
      const result = await request<{ devices?: Device[] }>('/v1/devices', {})
      return result.devices || []
    },
    createDevice(input: { siteId: string; name: string; serial: string }) {
      return request<Device>('/v1/devices', {
        method: 'POST',
        body: input,
        idempotent: true,
      })
    },
    updateDevice(device: Device, input: { siteId: string; name: string }) {
      return request<Device>(`/v1/devices/${encodeURIComponent(device.id)}`, {
        method: 'PATCH',
        body: { id: device.id, ...input, version: device.version },
        idempotent: true,
      })
    },
    deleteDevice(device: Device) {
      return request<void>(
        `/v1/devices/${encodeURIComponent(device.id)}?version=${encodeURIComponent(String(device.version))}`,
        {
          method: 'DELETE',
          idempotent: true,
        },
      )
    },
    transferDevice(device: Device, targetSiteId: string) {
      return request<Device>(`/v1/devices/${encodeURIComponent(device.id)}/transfer`, {
        method: 'PATCH',
        body: { id: device.id, targetSiteId, version: device.version },
        idempotent: true,
      })
    },
  }

  return { platformApi, tenantApi }
}
